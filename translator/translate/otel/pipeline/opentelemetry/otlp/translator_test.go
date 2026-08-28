// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	otlpreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

func TestNewTranslators(t *testing.T) {
	testCases := map[string]struct {
		input map[string]interface{}
		want  int
	}{
		"NilConf": {
			input: nil,
			want:  0,
		},
		"NoOtlpKey": {
			input: map[string]interface{}{},
			want:  0,
		},
		"WithOtlpKey": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"otlp": map[string]interface{}{
							"grpc_endpoint": "127.0.0.1:4317",
							"http_endpoint": "127.0.0.1:4318",
						},
					},
				},
			},
			want: 3, // metrics, logs, traces
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var conf *confmap.Conf
			if tc.input != nil {
				conf = confmap.NewFromStringMap(tc.input)
			}
			translators := NewTranslators(conf)
			assert.Equal(t, tc.want, translators.Len())
		})
	}
}

func TestOtlpPipelineTranslator(t *testing.T) {
	otlpreceiver.ClearConfigCache()
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"otlp": map[string]interface{}{
					"grpc_endpoint": "127.0.0.1:4317",
					"http_endpoint": "127.0.0.1:4318",
				},
			},
		},
	})

	translators := NewTranslators(conf)
	assert.Equal(t, 3, translators.Len())

	// Verify each signal pipeline
	signals := []pipeline.Signal{pipeline.SignalMetrics, pipeline.SignalLogs, pipeline.SignalTraces}
	for _, signal := range signals {
		t.Run(signal.String(), func(t *testing.T) {
			id := pipeline.NewIDWithName(signal, "otlp")
			translator, ok := translators.Get(id)
			require.True(t, ok)

			got, err := translator.Translate(conf)
			require.NoError(t, err)
			assert.Equal(t, 2, got.Receivers.Len())  // grpc + http
			assert.Equal(t, 1, got.Exporters.Len())  // forward connector
			assert.Equal(t, 1, got.Connectors.Len()) // forward connector
			if signal == pipeline.SignalLogs {
				assert.Equal(t, 2, got.Processors.Len())
				assert.Equal(t, "transform/otlp_scope", got.Processors.Keys()[0].String())
				assert.Equal(t, "transform/otlp_log_source", got.Processors.Keys()[1].String())
			} else {
				assert.Equal(t, 1, got.Processors.Len())
				assert.Equal(t, "transform/otlp_scope", got.Processors.Keys()[0].String())
			}
		})
	}
}
