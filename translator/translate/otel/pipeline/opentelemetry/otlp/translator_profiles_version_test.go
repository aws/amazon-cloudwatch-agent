// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/exporter/xexporter"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.opentelemetry.io/collector/service/pipelines"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/mem"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/service/configprovider"
	"github.com/aws/amazon-cloudwatch-agent/service/defaultcomponents"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
)

// The OTLP Profiles development version header, path and pipeline, spelled out
// here rather than taken from pprofileotlp or the translator so the tests pin
// the wire names independently.
const (
	profilesPath               = "/v1development/profiles"
	profilesPipelineName       = "profiles/otlp"
	profilesVersionHeader      = "OTLP-Profiles-Development-Version"
	profilesVersionHeaderLower = "otlp-profiles-development-version"
	profilesVersionHeaderUpper = "OTLP-PROFILES-DEVELOPMENT-VERSION"
)

type headerField struct {
	name  string
	value string
}

type profilesVersionCase struct {
	name string
	// header lists the version header fields to send, in order; nil sends no
	// version header. Names are used verbatim (not canonicalized) on HTTP and
	// lowercased on gRPC.
	header []headerField
	// accept is set on every case: true expects 200 / OK and one delivery to
	// the consumer, false expects 400 / INVALID_ARGUMENT and none.
	accept bool
}

// profilesVersionCases is shared by the HTTP and gRPC receiver tests: a missing
// header means version 1, exactly one value "1" is accepted, and anything else
// is rejected before the body is decoded.
var profilesVersionCases = []profilesVersionCase{
	{name: "Missing", header: nil, accept: true},
	{name: "One", header: []headerField{{profilesVersionHeader, "1"}}, accept: true},
	{name: "LowercaseNameOne", header: []headerField{{profilesVersionHeaderLower, "1"}}, accept: true},
	{name: "UppercaseNameOne", header: []headerField{{profilesVersionHeaderUpper, "1"}}, accept: true},
	{name: "LowercaseNameTwo", header: []headerField{{profilesVersionHeaderLower, "2"}}, accept: false},
	{name: "UppercaseNameZero", header: []headerField{{profilesVersionHeaderUpper, "0"}}, accept: false},
	{name: "Empty", header: []headerField{{profilesVersionHeader, ""}}, accept: false},
	{name: "Zero", header: []headerField{{profilesVersionHeader, "0"}}, accept: false},
	{name: "Two", header: []headerField{{profilesVersionHeader, "2"}}, accept: false},
	{name: "LeadingZero", header: []headerField{{profilesVersionHeader, "01"}}, accept: false},
	{name: "Decimal", header: []headerField{{profilesVersionHeader, "1.0"}}, accept: false},
	{name: "CommaList", header: []headerField{{profilesVersionHeader, "1,2"}}, accept: false},
	{name: "CommaListSame", header: []headerField{{profilesVersionHeader, "1, 1"}}, accept: false},
	{name: "RepeatedSame", header: []headerField{{profilesVersionHeader, "1"}, {profilesVersionHeader, "1"}}, accept: false},
	{name: "RepeatedDifferent", header: []headerField{{profilesVersionHeader, "1"}, {profilesVersionHeader, "2"}}, accept: false},
	{name: "RepeatedMixedNameCase", header: []headerField{{profilesVersionHeader, "1"}, {profilesVersionHeaderLower, "1"}}, accept: false},
}

// TestProfilesReceiverVersionHeaderHTTP builds the profiles pipeline's HTTP otlp
// receiver from the agent's config and sends real requests to
// /v1development/profiles.
func TestProfilesReceiverVersionHeaderHTTP(t *testing.T) {
	cases := append([]profilesVersionCase{}, profilesVersionCases...)
	// HTTP/1.1 strips optional whitespace around a field value (RFC 9110 5.5),
	// so " 1" reaches the receiver as "1". It is not distinguishable from "1"
	// on the wire and is accepted; the gRPC test covers the untrimmed value.
	cases = append(cases, profilesVersionCase{name: "LeadingSpaceTrimmedByHTTP", header: []headerField{{profilesVersionHeader, " 1"}}, accept: true})

	sink := new(consumertest.ProfilesSink)
	addr := startProfilesReceiver(t, httpProtocol, sink)
	client := &http.Client{Timeout: 10 * time.Second}
	send := func(t *testing.T, header []headerField, body []byte) (int, string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, "http://"+addr+profilesPath, bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-protobuf")
		for _, field := range header {
			// Assign the map entry directly so the name goes out with this exact casing.
			req.Header[field.name] = append(req.Header[field.name], field.value)
		}
		resp, err := client.Do(req)
		require.NoError(t, err)
		respBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		return resp.StatusCode, strings.ToLower(string(respBody))
	}

	// Control: an undecodable body with a valid version fails with a decode
	// error that does not name the header, so the reject cases below can tell
	// a version rejection from a decode failure.
	t.Run("UndecodableBodyControl", func(t *testing.T) {
		sink.Reset()
		code, msg := send(t, []headerField{{profilesVersionHeader, "1"}}, undecodableBody)
		assert.Equal(t, http.StatusBadRequest, code)
		assert.NotContains(t, msg, profilesVersionHeaderLower)
		assert.Empty(t, sink.AllProfiles())
	})

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.accept {
				sink.Reset()
				code, msg := send(t, testCase.header, minimalProfilesRequest(t))
				assert.Equal(t, http.StatusOK, code, msg)
				assert.Len(t, sink.AllProfiles(), 1)
				return
			}
			for _, body := range rejectBodies(t) {
				t.Run(body.name, func(t *testing.T) {
					sink.Reset()
					code, msg := send(t, testCase.header, body.data)
					assert.Equal(t, http.StatusBadRequest, code)
					// The error names the header even for an undecodable body,
					// so the version is checked before the body is decoded.
					assert.Contains(t, msg, profilesVersionHeaderLower)
					assert.Empty(t, sink.AllProfiles(), "consumer must not be called for a rejected request")
				})
			}
		})
	}
}

// TestProfilesReceiverVersionHeaderGRPC runs the same cases against
// ProfilesService/Export with the header sent as gRPC metadata.
func TestProfilesReceiverVersionHeaderGRPC(t *testing.T) {
	cases := append([]profilesVersionCase{}, profilesVersionCases...)
	cases = append(cases, profilesVersionCase{name: "LeadingSpace", header: []headerField{{profilesVersionHeader, " 1"}}, accept: false})

	sink := new(consumertest.ProfilesSink)
	addr := startProfilesReceiver(t, grpcProtocol, sink)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, conn.Close()) })
	send := func(t *testing.T, header []headerField, body []byte) error {
		t.Helper()
		md := metadata.MD{}
		for _, field := range header {
			// Append lowercases the key, as every gRPC client must, so the
			// casing rows exercise the same metadata as their lowercase peers.
			md.Append(field.name, field.value)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ctx = metadata.NewOutgoingContext(ctx, md)
		// Invoke the method directly: pprofileotlp's gRPC client replaces any
		// caller-supplied version with "1", so it cannot send the bad values.
		var resp []byte
		return conn.Invoke(ctx, profilesExportMethod, body, &resp, grpc.ForceCodecV2(rawCodec{}))
	}

	t.Run("UndecodableBodyControl", func(t *testing.T) {
		sink.Reset()
		st, ok := status.FromError(send(t, []headerField{{profilesVersionHeader, "1"}}, undecodableBody))
		require.True(t, ok)
		assert.NotEqual(t, codes.OK, st.Code())
		assert.NotContains(t, st.Message(), profilesVersionHeaderLower)
		assert.Empty(t, sink.AllProfiles())
	})

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.accept {
				sink.Reset()
				assert.NoError(t, send(t, testCase.header, minimalProfilesRequest(t)))
				assert.Len(t, sink.AllProfiles(), 1)
				return
			}
			for _, body := range rejectBodies(t) {
				t.Run(body.name, func(t *testing.T) {
					sink.Reset()
					err := send(t, testCase.header, body.data)
					st, ok := status.FromError(err)
					require.True(t, ok, "%v", err)
					assert.Equal(t, codes.InvalidArgument, st.Code(), st.Message())
					// Named even for an undecodable body: checked before decoding.
					assert.Contains(t, st.Message(), profilesVersionHeaderLower)
					assert.Empty(t, sink.AllProfiles(), "consumer must not be called for a rejected request")
				})
			}
		})
	}
}

// TestProfilesExporterVersionHeader builds the profiles pipeline's exporter and
// its SigV4 auth chain from the agent's config, points it at a local server and
// checks that the version header is sent and signed.
func TestProfilesExporterVersionHeader(t *testing.T) {
	// Static fake credentials from the environment; isolate from any shared
	// config or credentials file and from instance metadata.
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_CA_BUNDLE", "")

	factories, agentCfg := loadAgentConfig(t)

	requests := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		// Keep the first request; never block the handler on later ones.
		select {
		case requests <- r:
		default:
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	exporterIDs := profilesPipeline(t, agentCfg).Exporters
	require.Len(t, exporterIDs, 1)
	exporterID := exporterIDs[0]
	assert.Equal(t, "otlp_http/profiles", exporterID.String())
	factory, ok := factories.Exporters[exporterID.Type()].(xexporter.Factory)
	require.True(t, ok)
	exporterCfg, ok := agentCfg.Exporters[exporterID].(*otlphttpexporter.Config)
	require.True(t, ok)
	require.True(t, strings.HasSuffix(exporterCfg.ProfilesEndpoint, profilesPath), exporterCfg.ProfilesEndpoint)
	require.True(t, exporterCfg.ClientConfig.Auth.HasValue(), "the agent's exporter must be authenticated")

	// Follow the auth chain the agent configures (agenthealth, which wraps
	// sigv4auth as its additional auth) and start it innermost first, since
	// agenthealth looks its additional auth up from the host when it starts.
	authenticator := exporterCfg.ClientConfig.Auth.Get().AuthenticatorID
	healthCfg, ok := agentCfg.Extensions[authenticator].(*agenthealth.Config)
	require.True(t, ok, authenticator.String())
	require.NotNil(t, healthCfg.AdditionalAuth, "agenthealth must chain to sigv4auth")
	host := &extensionsHost{extensions: map[component.ID]component.Component{}}
	for _, id := range []component.ID{*healthCfg.AdditionalAuth, authenticator} {
		factory := factories.Extensions[id.Type()]
		require.NotNil(t, factory, id.String())
		ext, err := factory.Create(context.Background(), extensiontest.NewNopSettings(id.Type()), agentCfg.Extensions[id])
		require.NoError(t, err)
		require.NoError(t, ext.Start(context.Background(), host))
		t.Cleanup(func() { assert.NoError(t, ext.Shutdown(context.Background())) })
		host.extensions[id] = ext
	}

	// Only the destination changes; headers, auth and everything else are as the agent loads them.
	exporterCfg.ProfilesEndpoint = server.URL + profilesPath

	exp, err := factory.CreateProfiles(context.Background(), exportertest.NewNopSettings(exporterID.Type()), exporterCfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(context.Background(), host))
	t.Cleanup(func() { assert.NoError(t, exp.Shutdown(context.Background())) })
	require.NoError(t, exp.ConsumeProfiles(context.Background(), minimalProfiles()))

	var req *http.Request
	select {
	case req = <-requests:
	case <-time.After(30 * time.Second):
		require.FailNow(t, "exporter did not send a request")
	}
	assert.Equal(t, profilesPath, req.URL.Path)
	assert.Equal(t, []string{"1"}, req.Header.Values(profilesVersionHeader))

	authorization := req.Header.Get("Authorization")
	require.True(t, strings.HasPrefix(authorization, "AWS4-HMAC-SHA256 "), "request must be SigV4 signed: %q", authorization)
	assert.Contains(t, authorization, "/us-west-2/monitoring/aws4_request")
	assert.Contains(t, signedHeaders(t, authorization), profilesVersionHeaderLower)
}

type otlpProtocol string

const (
	httpProtocol otlpProtocol = "http"
	grpcProtocol otlpProtocol = "grpc"
)

// startProfilesReceiver builds the profiles pipeline's otlp receiver for the
// given protocol from the agent's config, moves it to a free localhost port,
// creates it with the agent's factories and starts it. It returns the address.
func startProfilesReceiver(t *testing.T, protocol otlpProtocol, next *consumertest.ProfilesSink) string {
	t.Helper()
	factories, agentCfg := loadAgentConfig(t)
	id, cfg := profilesReceiver(t, agentCfg, protocol)
	factory, ok := factories.Receivers[id.Type()].(xreceiver.Factory)
	require.True(t, ok, id.String())

	// A port picked from a closed listener can be taken before the receiver
	// binds it; retry with a new port rather than fail on that race.
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		addr := freeLocalAddress(t)
		if protocol == httpProtocol {
			cfg.Protocols.HTTP.Get().ServerConfig.NetAddr.Endpoint = addr
		} else {
			cfg.Protocols.GRPC.Get().NetAddr.Endpoint = addr
		}
		rcv, err := factory.CreateProfiles(context.Background(), receivertest.NewNopSettings(id.Type()), cfg, next)
		require.NoError(t, err)
		if lastErr = rcv.Start(context.Background(), &extensionsHost{}); lastErr != nil {
			// The bind error differs by platform, so retry any start error;
			// a persistent one still fails below after the last attempt.
			_ = rcv.Shutdown(context.Background())
			continue
		}
		t.Cleanup(func() { assert.NoError(t, rcv.Shutdown(context.Background())) })
		return addr
	}
	require.NoError(t, lastErr, "no free port after retries")
	return ""
}

// loadAgentConfig loads the translator's golden config for the otlp section the
// way the agent does at startup, as service/configprovider's profiles queue test
// does: written with the agent's YAML writer, then resolved through the agent's
// config provider settings (including its otlphttp endpoint validator) with the
// agent's component factories. The translator's golden tests keep that file in
// step with the profiles pipeline translator.
func loadAgentConfig(t *testing.T) (otelcol.Factories, *otelcol.Config) {
	t.Helper()
	factories, err := defaultcomponents.Factories()
	require.NoError(t, err)
	golden, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "tocwconfig", "sampleConfig", "opentelemetry", "otlp_otel_config.yaml"))
	require.NoError(t, err)
	var cfgMap map[string]any
	require.NoError(t, yaml.Unmarshal(golden, &cfgMap))
	yamlPath := filepath.Join(t.TempDir(), "otlp_otel_config.yaml")
	require.NoError(t, cmdutil.ConfigToYamlFile(cfgMap, yamlPath))
	provider, err := otelcol.NewConfigProvider(configprovider.GetSettings([]string{yamlPath}, zap.NewNop()))
	require.NoError(t, err)
	cfg, err := provider.Get(context.Background(), factories)
	require.NoError(t, err)
	return factories, cfg
}

func profilesPipeline(t *testing.T, cfg *otelcol.Config) *pipelines.PipelineConfig {
	t.Helper()
	for id, pipelineCfg := range cfg.Service.Pipelines {
		if id.String() == profilesPipelineName {
			return pipelineCfg
		}
	}
	require.FailNow(t, "agent config has no profiles pipeline", profilesPipelineName)
	return nil
}

// profilesReceiver returns the profiles pipeline's otlp receiver that serves
// the given protocol.
func profilesReceiver(t *testing.T, cfg *otelcol.Config, protocol otlpProtocol) (component.ID, *otlpreceiver.Config) {
	t.Helper()
	for _, id := range profilesPipeline(t, cfg).Receivers {
		receiverCfg, ok := cfg.Receivers[id].(*otlpreceiver.Config)
		if !ok {
			continue
		}
		if (protocol == httpProtocol && receiverCfg.Protocols.HTTP.HasValue()) ||
			(protocol == grpcProtocol && receiverCfg.Protocols.GRPC.HasValue()) {
			return id, receiverCfg
		}
	}
	require.FailNow(t, "no otlp receiver for protocol in the profiles pipeline", string(protocol))
	return component.ID{}, nil
}

func signedHeaders(t *testing.T, authorization string) []string {
	t.Helper()
	for _, part := range strings.Split(strings.TrimPrefix(authorization, "AWS4-HMAC-SHA256 "), ",") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(part), "SignedHeaders="); ok {
			return strings.Split(value, ";")
		}
	}
	require.FailNow(t, "no SignedHeaders in Authorization", authorization)
	return nil
}

func freeLocalAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	return addr
}

func minimalProfiles() pprofile.Profiles {
	profiles := pprofile.NewProfiles()
	profile := profiles.ResourceProfiles().AppendEmpty().ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
	profile.Samples().AppendEmpty().Values().Append(1)
	return profiles
}

// undecodableBody is a truncated protobuf field (field 1, length 5, one byte
// of data) that no ExportProfilesServiceRequest decoder accepts.
var undecodableBody = []byte{0x0a, 0x05, 0x01}

// rejectBodies are sent with every rejected version: a valid body, and one
// that cannot be decoded, which still has to be rejected for its version.
func rejectBodies(t *testing.T) []namedBody {
	return []namedBody{
		{name: "ValidBody", data: minimalProfilesRequest(t)},
		{name: "UndecodableBody", data: undecodableBody},
	}
}

type namedBody struct {
	name string
	data []byte
}

func minimalProfilesRequest(t *testing.T) []byte {
	t.Helper()
	body, err := pprofileotlp.NewExportRequestFromProfiles(minimalProfiles()).MarshalProto()
	require.NoError(t, err)
	return body
}

const profilesExportMethod = "/opentelemetry.proto.collector.profiles.v1development.ProfilesService/Export"

// rawCodec sends and receives already-encoded protobuf bytes unchanged. Its
// name is "proto" so the request carries the standard application/grpc+proto
// content type and the server decodes it with its own protobuf codec.
type rawCodec struct{}

func (rawCodec) Marshal(v any) (mem.BufferSlice, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("rawCodec: unexpected request type %T", v)
	}
	return mem.BufferSlice{mem.SliceBuffer(b)}, nil
}

func (rawCodec) Unmarshal(data mem.BufferSlice, v any) error {
	b, ok := v.(*[]byte)
	if !ok {
		return fmt.Errorf("rawCodec: unexpected response type %T", v)
	}
	*b = data.Materialize()
	return nil
}

func (rawCodec) Name() string { return "proto" }

type extensionsHost struct {
	extensions map[component.ID]component.Component
}

var _ component.Host = (*extensionsHost)(nil)

func (h *extensionsHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}
