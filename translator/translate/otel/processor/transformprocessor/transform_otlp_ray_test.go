// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package transformprocessor

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// TestOtlpRayStatements runs the rendered otlp_ray statements over spans and checks the
// resource each one leaves for transform/identity.
func TestOtlpRayStatements(t *testing.T) {
	cfg, err := NewTranslatorWithName(common.OtlpRay).Translate(nil)
	require.NoError(t, err)

	tests := map[string]struct {
		in   map[string]string
		want map[string]string
		gone []string
	}{
		"Ray pod with the SDK fallback name": {
			in: map[string]string{
				"service.name":                 "unknown_service",
				"app.kubernetes.io/name":       "kuberay",
				"k8s.pod.label.ray.io/cluster": "llm-raycluster-abcde",
			},
			want: map[string]string{"k8s.pod.label.ray.io/cluster": "llm-raycluster-abcde"},
			gone: []string{"service.name", "app.kubernetes.io/name"},
		},
		"Ray pod with an executable-suffixed fallback name": {
			in: map[string]string{
				"service.name":                 "unknown_service:python3",
				"k8s.pod.label.ray.io/cluster": "llm-raycluster-abcde",
			},
			gone: []string{"service.name"},
		},
		"Ray pod with a customer-set name": {
			in: map[string]string{
				"service.name":                 "chat-api",
				"app.kubernetes.io/name":       "kuberay",
				"k8s.pod.label.ray.io/cluster": "llm-raycluster-abcde",
			},
			want: map[string]string{"service.name": "chat-api"},
			gone: []string{"app.kubernetes.io/name"},
		},
		"Ray pod with a customer app.kubernetes.io/name": {
			in: map[string]string{
				"app.kubernetes.io/name":       "chat",
				"k8s.pod.label.ray.io/cluster": "llm-raycluster-abcde",
			},
			want: map[string]string{"app.kubernetes.io/name": "chat"},
		},
		"non-Ray pod is untouched": {
			in: map[string]string{
				"service.name":           "unknown_service",
				"app.kubernetes.io/name": "kuberay",
			},
			want: map[string]string{"service.name": "unknown_service", "app.kubernetes.io/name": "kuberay"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			sink := new(consumertest.TracesSink)
			proc, err := transformprocessor.NewFactory().CreateTraces(context.Background(),
				processortest.NewNopSettings(transformprocessor.NewFactory().Type()), cfg, sink)
			require.NoError(t, err)
			require.NoError(t, proc.Start(context.Background(), componenttest.NewNopHost()))
			t.Cleanup(func() { require.NoError(t, proc.Shutdown(context.Background())) })

			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			for k, v := range tc.in {
				rs.Resource().Attributes().PutStr(k, v)
			}
			rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("proxy_http_request")
			require.NoError(t, proc.ConsumeTraces(context.Background(), td))

			require.Len(t, sink.AllTraces(), 1)
			attrs := sink.AllTraces()[0].ResourceSpans().At(0).Resource().Attributes()
			for k, v := range tc.want {
				got, ok := attrs.Get(k)
				if assert.True(t, ok, "%s kept", k) {
					assert.Equal(t, v, got.Str(), k)
				}
			}
			for _, k := range tc.gone {
				_, ok := attrs.Get(k)
				assert.False(t, ok, "%s removed", k)
			}
		})
	}
}
