// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// ResetRegistry clears the shared receiver registry for testing
func ResetRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[EndpointConfig]common.ComponentTranslator)
}

func TestTranslatorWithoutDataType(t *testing.T) {
	tt := NewTranslator()
	assert.EqualValues(t, "otlp", tt.ID().String())
	got, err := tt.Translate(confmap.New())
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestTracesTranslator(t *testing.T) {
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *confmap.Conf
		wantErr bool
	}{
		"WithMissingKey": {
			input:   map[string]interface{}{"logs": map[string]interface{}{}},
			wantErr: true,
		},
		"WithDefault": {
			input: map[string]interface{}{"traces": map[string]interface{}{"traces_collected": map[string]interface{}{"otlp": map[string]interface{}{}}}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"protocols": map[string]interface{}{
					"grpc": map[string]interface{}{
						"endpoint": "127.0.0.1:4317",
					},
					"http": map[string]interface{}{
						"endpoint": "127.0.0.1:4318",
					},
				},
			}),
		},
		"WithCompleteConfig": {
			input: testutil.GetJson(t, filepath.Join("testdata", "traces", "config.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "traces", "config.yaml")),
		},
	}
	factory := otlpreceiver.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			tt := NewTranslator(WithSignal(pipeline.SignalTraces), WithConfigKey(common.ConfigKey(common.TracesKey, common.TracesCollectedKey, common.OtlpKey)))
			got, err := tt.Translate(conf)
			if testCase.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			require.NotNil(t, got)
			gotCfg, ok := got.(*otlpreceiver.Config)
			require.True(t, ok)
			wantCfg := factory.CreateDefaultConfig()
			require.NoError(t, testCase.want.Unmarshal(wantCfg))
			assert.Equal(t, wantCfg, gotCfg)
		})
	}
}

func TestSharedTranslatorDeduplication(t *testing.T) {
	ResetRegistry()

	config := map[string]interface{}{
		"metrics": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"otlp": map[string]interface{}{
					"grpc_endpoint": "127.0.0.1:4317",
					"http_endpoint": "127.0.0.1:4318",
				},
			},
		},
		"traces": map[string]interface{}{
			"traces_collected": map[string]interface{}{
				"otlp": map[string]interface{}{
					"grpc_endpoint": "127.0.0.1:4317",
					"http_endpoint": "127.0.0.1:4318",
				},
			},
		},
	}

	conf := confmap.NewFromStringMap(config)

	metricsTranslator := NewTranslator(
		WithSignal(pipeline.SignalMetrics),
		WithConfigKey(common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.OtlpKey)),
	)
	tracesTranslator := NewTranslator(
		WithSignal(pipeline.SignalTraces),
		WithConfigKey(common.ConfigKey(common.TracesKey, common.TracesCollectedKey, common.OtlpKey)),
	)

	// Translate to trigger registry logic
	_, err := metricsTranslator.Translate(conf)
	require.NoError(t, err)
	_, err = tracesTranslator.Translate(conf)
	require.NoError(t, err)

	assert.Equal(t, metricsTranslator.ID(), tracesTranslator.ID(), "Same endpoints should share receiver after translation")

	_, err1 := metricsTranslator.Translate(conf)
	_, err2 := tracesTranslator.Translate(conf)
	assert.NoError(t, err1)
	assert.NoError(t, err2)
}

func TestPortConflictDetection(t *testing.T) {
	ResetRegistry()

	config := map[string]interface{}{
		"metrics": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"otlp": map[string]interface{}{
					"grpc_endpoint": "127.0.0.1:4317",
					"http_endpoint": "127.0.0.1:4318",
					"tls": map[string]interface{}{
						"cert_file": "/path/to/cert1.pem",
						"key_file":  "/path/to/key1.pem",
					},
				},
			},
		},
		"traces": map[string]interface{}{
			"traces_collected": map[string]interface{}{
				"otlp": map[string]interface{}{
					"grpc_endpoint": "127.0.0.1:4317",
					"http_endpoint": "127.0.0.1:4318",
					"tls": map[string]interface{}{
						"cert_file": "/path/to/cert2.pem",
						"key_file":  "/path/to/key2.pem",
					},
				},
			},
		},
	}

	conf := confmap.NewFromStringMap(config)

	metricsTranslator := NewTranslator(
		WithSignal(pipeline.SignalMetrics),
		WithConfigKey(common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.OtlpKey)),
	)
	_, err1 := metricsTranslator.Translate(conf)
	assert.NoError(t, err1, "First translator should succeed")

	tracesTranslator := NewTranslator(
		WithSignal(pipeline.SignalTraces),
		WithConfigKey(common.ConfigKey(common.TracesKey, common.TracesCollectedKey, common.OtlpKey)),
	)
	_, err2 := tracesTranslator.Translate(conf)
	assert.Error(t, err2, "Second translator should fail due to port conflict")
	assert.Contains(t, err2.Error(), "port conflict", "Error should mention port conflict")
}
