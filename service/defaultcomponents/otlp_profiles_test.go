// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package defaultcomponents

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestProfilesReceiverResolvesReferencesForNextProcessor(t *testing.T) {
	factories, err := Factories()
	require.NoError(t, err)
	payloads := map[string][]byte{
		"synthetic":    dictionaryEncoded(t),
		"ebpfProfiler": readTestdata(t, "ebpf-profiler-payload.pb"),
	}
	for _, transport := range []string{"http", "grpc"} {
		sink := new(consumertest.ProfilesSink)
		addr := startProfilesReceiver(t, factories, newResourceProcessor(t, factories, sink), transport)
		for name, payload := range payloads {
			t.Run(transport+"/"+name, func(t *testing.T) {
				sink.Reset()
				require.NoError(t, exportProfiles(t, transport, addr, payload))
				require.Len(t, sink.AllProfiles(), 1)
				got := sink.AllProfiles()[0]
				resource := got.ResourceProfiles().At(0).Resource().Attributes()
				for key, value := range resource.All() {
					assert.NotEmpty(t, key)
					assert.NotEqual(t, pcommon.ValueTypeEmpty, value.Type(), key)
				}
				assertStr(t, resource, "k8s.pod.ip", "10.42.0.13")
				pid, ok := resource.Get("process.pid")
				require.True(t, ok)
				assert.Equal(t, pcommon.ValueTypeInt, pid.Type())
				if name == "synthetic" {
					assertStr(t, resource, "service.name", "payments")
					assertStr(t, got.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes(), "cloudwatch.source", "ebpf")
				} else {
					assertStr(t, resource, "service.name", "unknown_service")
					assertStr(t, resource, "container.id", "eb6e587a2312434efabb3c9cf78a35376ee463f301b5732e5b4afefa98927456")
					assert.Equal(t, 2, got.ResourceProfiles().Len())
				}
			})
		}
		t.Run(transport+"/invalidReference", func(t *testing.T) {
			sink.Reset()
			err := exportProfiles(t, transport, addr, invalidKeyReference())
			if transport == "http" {
				assert.Equal(t, &httpStatusError{code: http.StatusBadRequest}, err)
			} else {
				assert.Equal(t, codes.InvalidArgument, status.Code(err))
				assert.ErrorContains(t, err, "invalid key_strindex")
			}
			assert.Empty(t, sink.AllProfiles())
		})
	}
}

func newProfiles() pprofile.Profiles {
	profiles := pprofile.NewProfiles()
	resourceProfiles := profiles.ResourceProfiles().AppendEmpty()
	resourceProfiles.Resource().Attributes().PutStr("service.name", "payments")
	resourceProfiles.Resource().Attributes().PutStr("k8s.pod.ip", "10.42.0.13")
	resourceProfiles.Resource().Attributes().PutStr("container.id", "eb6e587a2312434efabb3c9cf78a35376ee463f301b5732e5b4afefa98927456")
	resourceProfiles.Resource().Attributes().PutInt("process.pid", 34915)
	scopeProfiles := resourceProfiles.ScopeProfiles().AppendEmpty()
	scopeProfiles.Scope().Attributes().PutStr("cloudwatch.source", "ebpf")
	scopeProfiles.Profiles().AppendEmpty().Samples().AppendEmpty()
	return profiles
}

func dictionaryEncoded(t *testing.T) []byte {
	t.Helper()
	profiles := newProfiles()
	profiles.Dictionary().StringTable().Append("")
	buf, err := (&pprofile.ProtoMarshaler{}).MarshalProfiles(profiles)
	require.NoError(t, err)
	return buf
}

// invalidKeyReference is an export request whose only resource attribute has
// key_strindex 1 and no dictionary string table:
// resource_profiles { resource { attributes { key_strindex: 1 } } }.
func invalidKeyReference() []byte {
	return []byte{0x0a, 0x06, 0x0a, 0x04, 0x0a, 0x02, 0x18, 0x01}
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return buf
}

func startProfilesReceiver(t *testing.T, factories otelcol.Factories, next xconsumer.Profiles, transport string) string {
	t.Helper()
	factory, ok := factories.Receivers[component.MustNewType("otlp")].(xreceiver.Factory)
	require.True(t, ok)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	cfg := factory.CreateDefaultConfig().(*otlpreceiver.Config)
	switch transport {
	case "http":
		cfg.Protocols.GRPC = configoptional.None[configgrpc.ServerConfig]()
		cfg.Protocols.HTTP.GetOrInsertDefault().ServerConfig.NetAddr.Endpoint = addr
	case "grpc":
		cfg.Protocols.HTTP = configoptional.None[otlpreceiver.HTTPConfig]()
		cfg.Protocols.GRPC.GetOrInsertDefault().NetAddr.Endpoint = addr
	default:
		t.Fatalf("unknown transport %q", transport)
	}
	recv, err := factory.CreateProfiles(context.Background(), receivertest.NewNopSettings(factory.Type()), cfg, next)
	require.NoError(t, err)
	require.NoError(t, recv.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, recv.Shutdown(context.Background())) })
	return addr
}

func newResourceProcessor(t *testing.T, factories otelcol.Factories, next xconsumer.Profiles) xconsumer.Profiles {
	t.Helper()
	factory, ok := factories.Processors[component.MustNewType("resource")].(xprocessor.Factory)
	require.True(t, ok)
	cfg := factory.CreateDefaultConfig()
	conf := confmap.NewFromStringMap(map[string]any{"attributes": []any{map[string]any{"key": "service.name", "value": "unknown_service", "action": "insert"}}})
	require.NoError(t, conf.Unmarshal(cfg))
	proc, err := factory.CreateProfiles(context.Background(), processortest.NewNopSettings(factory.Type()), cfg, next)
	require.NoError(t, err)
	require.NoError(t, proc.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, proc.Shutdown(context.Background())) })
	return proc
}

func exportProfiles(t *testing.T, transport, addr string, payload []byte) error {
	t.Helper()
	if transport == "http" {
		resp, err := http.Post("http://"+addr+"/v1development/profiles", "application/x-protobuf", bytes.NewReader(payload))
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		if resp.StatusCode != http.StatusOK {
			return &httpStatusError{code: resp.StatusCode}
		}
		return nil
	}
	clientCfg := configgrpc.NewDefaultClientConfig()
	clientCfg.Endpoint = addr
	clientCfg.TLS.Insecure = true
	conn, err := clientCfg.ToClientConn(context.Background(), nil, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	var response []byte
	return conn.Invoke(context.Background(), "/opentelemetry.proto.collector.profiles.v1development.ProfilesService/Export", payload, &response, grpc.ForceCodec(rawCodec{}))
}

type httpStatusError struct{ code int }

func (e *httpStatusError) Error() string { return http.StatusText(e.code) }

type rawCodec struct{}

func (rawCodec) Marshal(v any) ([]byte, error) { return v.([]byte), nil }

func (rawCodec) Unmarshal(data []byte, v any) error {
	*(v.(*[]byte)) = data
	return nil
}

func (rawCodec) Name() string { return "proto" }

func assertStr(t *testing.T, attributes pcommon.Map, key, want string) {
	t.Helper()
	value, ok := attributes.Get(key)
	require.True(t, ok, key)
	assert.Equal(t, want, value.Str(), key)
}
