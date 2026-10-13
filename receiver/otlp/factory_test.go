// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestReceiverPathLeavesReferencesUnresolved(t *testing.T) {
	request := pprofileotlp.NewExportRequest()
	require.NoError(t, request.UnmarshalProto(dictionaryEncoded(t)))
	attributes := request.Profiles().ResourceProfiles().At(0).Resource().Attributes()
	require.Equal(t, 4, attributes.Len())
	for key, value := range attributes.All() {
		assert.Empty(t, key)
		if value.Type() != pcommon.ValueTypeInt {
			assert.Equal(t, pcommon.ValueTypeEmpty, value.Type())
		}
	}
}

func TestProfilesReceiverResolvesReferencesForNextProcessor(t *testing.T) {
	payloads := map[string][]byte{
		"synthetic":    dictionaryEncoded(t),
		"ebpfProfiler": readTestdata(t, "ebpf-profiler-payload.pb"),
	}
	for _, transport := range []string{"http", "grpc"} {
		sink := new(consumertest.ProfilesSink)
		addr := startProfilesReceiver(t, newResourceProcessor(t, sink), transport)
		for name, payload := range payloads {
			t.Run(transport+"/"+name, func(t *testing.T) {
				sink.Reset()
				exportProfiles(t, transport, addr, payload)
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
	}
}

func TestResolveReferencesPassesPlainProfilesThrough(t *testing.T) {
	profiles := newProfiles()
	assert.Equal(t, profiles, resolveReferences(zap.NewNop(), profiles))
	assert.Equal(t, 0, profiles.Dictionary().StringTable().Len())
}

func TestResolveReferencesKeepsSamples(t *testing.T) {
	request := pprofileotlp.NewExportRequest()
	require.NoError(t, request.UnmarshalProto(readTestdata(t, "ebpf-profiler-payload.pb")))
	want := request.Profiles().SampleCount()
	wantStrings := request.Profiles().Dictionary().StringTable().Len()
	got := resolveReferences(zap.NewNop(), request.Profiles())
	assert.Equal(t, want, got.SampleCount())
	assert.Equal(t, wantStrings, got.Dictionary().StringTable().Len())
}

func TestResolvedProfilesExportInlineAttributes(t *testing.T) {
	testCases := map[string]struct {
		payload  []byte
		wantKeys []string
	}{
		"synthetic":    {payload: dictionaryEncoded(t), wantKeys: []string{"service.name", "k8s.pod.ip", "cloudwatch.source"}},
		"ebpfProfiler": {payload: readTestdata(t, "ebpf-profiler-payload.pb"), wantKeys: []string{"k8s.pod.ip", "container.id"}},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			request := pprofileotlp.NewExportRequest()
			require.NoError(t, request.UnmarshalProto(testCase.payload))
			exported, err := pprofileotlp.NewExportRequestFromProfiles(resolveReferences(zap.NewNop(), request.Profiles())).MarshalProto()
			require.NoError(t, err)

			var keys []string
			for _, resourceProfiles := range messageFields(t, exported, 1) {
				for _, resource := range messageFields(t, resourceProfiles, 1) {
					keys = append(keys, inlineAttributeKeys(t, resource, 1)...)
				}
				for _, scopeProfiles := range messageFields(t, resourceProfiles, 2) {
					for _, scope := range messageFields(t, scopeProfiles, 1) {
						keys = append(keys, inlineAttributeKeys(t, scope, 3)...)
					}
				}
			}
			for _, key := range testCase.wantKeys {
				assert.Contains(t, keys, key)
			}
		})
	}
}

// inlineAttributeKeys returns the keys of the repeated KeyValue field at the given number
// (Resource.attributes is 1, InstrumentationScope.attributes is 3) and fails if any
// KeyValue also carries key_strindex.
func inlineAttributeKeys(t *testing.T, message []byte, attributes protowire.Number) []string {
	t.Helper()
	var keys []string
	for _, keyValue := range messageFields(t, message, attributes) {
		assert.Empty(t, messageFields(t, keyValue, 3), "key_strindex set alongside key")
		for _, key := range messageFields(t, keyValue, 1) {
			keys = append(keys, string(key))
		}
	}
	return keys
}

// messageFields returns the raw payload of every occurrence of a length-delimited or
// varint field in a protobuf message.
func messageFields(t *testing.T, message []byte, want protowire.Number) [][]byte {
	t.Helper()
	var values [][]byte
	for len(message) > 0 {
		number, typ, n := protowire.ConsumeTag(message)
		require.Positive(t, n)
		message = message[n:]
		if typ == protowire.BytesType {
			value, n := protowire.ConsumeBytes(message)
			require.Positive(t, n)
			if number == want {
				values = append(values, value)
			}
			message = message[n:]
			continue
		}
		n = protowire.ConsumeFieldValue(number, typ, message)
		require.Positive(t, n)
		if number == want {
			values = append(values, message[:n])
		}
		message = message[n:]
	}
	return values
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

// dictionaryEncoded marshals through pprofile.ProtoMarshaler, which replaces attribute
// strings with ProfilesDictionary string-table references the way the eBPF profiler does.
func dictionaryEncoded(t *testing.T) []byte {
	t.Helper()
	profiles := newProfiles()
	profiles.Dictionary().StringTable().Append("")
	buf, err := (&pprofile.ProtoMarshaler{}).MarshalProfiles(profiles)
	require.NoError(t, err)
	return buf
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return buf
}

func startProfilesReceiver(t *testing.T, next xconsumer.Profiles, transport string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	factory := NewFactory().(xreceiver.Factory)
	cfg := factory.CreateDefaultConfig().(*otlpreceiver.Config)
	switch transport {
	case "http":
		cfg.GRPC = configoptional.None[configgrpc.ServerConfig]()
		cfg.HTTP.GetOrInsertDefault().ServerConfig.NetAddr.Endpoint = addr
	case "grpc":
		cfg.HTTP = configoptional.None[otlpreceiver.HTTPConfig]()
		cfg.GRPC.GetOrInsertDefault().NetAddr.Endpoint = addr
	default:
		t.Fatalf("unknown transport %q", transport)
	}
	recv, err := factory.CreateProfiles(context.Background(), receivertest.NewNopSettings(factory.Type()), cfg, next)
	require.NoError(t, err)
	require.NoError(t, recv.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, recv.Shutdown(context.Background())) })
	return addr
}

func newResourceProcessor(t *testing.T, next xconsumer.Profiles) xconsumer.Profiles {
	t.Helper()
	factory := resourceprocessor.NewFactory().(xprocessor.Factory)
	cfg := factory.CreateDefaultConfig().(*resourceprocessor.Config)
	conf := confmap.NewFromStringMap(map[string]any{"attributes": []any{map[string]any{"key": "service.name", "value": "unknown_service", "action": "insert"}}})
	require.NoError(t, conf.Unmarshal(cfg))
	proc, err := factory.CreateProfiles(context.Background(), processortest.NewNopSettings(factory.Type()), cfg, next)
	require.NoError(t, err)
	require.NoError(t, proc.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, proc.Shutdown(context.Background())) })
	return proc
}

func exportProfiles(t *testing.T, transport, addr string, payload []byte) {
	t.Helper()
	if transport == "http" {
		resp, err := http.Post("http://"+addr+"/v1development/profiles", "application/x-protobuf", bytes.NewReader(payload))
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusOK, resp.StatusCode)
		return
	}
	clientCfg := configgrpc.NewDefaultClientConfig()
	clientCfg.Endpoint = addr
	clientCfg.TLS.Insecure = true
	conn, err := clientCfg.ToClientConn(context.Background(), nil, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	request := pprofileotlp.NewExportRequest()
	require.NoError(t, request.UnmarshalProto(payload))
	_, err = pprofileotlp.NewGRPCClient(conn).Export(context.Background(), request)
	require.NoError(t, err)
}

func assertStr(t *testing.T, attributes pcommon.Map, key, want string) {
	t.Helper()
	value, ok := attributes.Get(key)
	require.True(t, ok, key)
	assert.Equal(t, want, value.Str(), key)
}
