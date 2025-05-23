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

func TestTranslatorWithoutDataType(t *testing.T) {
	tt := NewTranslator()
	assert.EqualValues(t, "otlp", tt.ID().String())
	got, err := tt.Translate(confmap.New())
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestTracesTranslator(t *testing.T) {
	tt := NewTranslator(WithSignal(pipeline.SignalTraces), WithConfigKey(common.ConfigKey(common.TracesKey, common.TracesCollectedKey, common.OtlpKey)))
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *confmap.Conf
		wantErr error
	}{
		"WithMissingKey": {
			input: map[string]interface{}{"logs": map[string]interface{}{}},
			wantErr: &common.MissingKeyError{
				ID:      tt.ID(),
				JsonKey: common.ConfigKey(common.TracesKey, common.TracesCollectedKey, common.OtlpKey),
			},
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
		"WithTLS": {
			input: map[string]interface{}{
				"protocols": map[string]interface{}{
					"grpc": map[string]interface{}{
						"endpoint": "127.0.0.1:4317",
					},
					"http": map[string]interface{}{
						"endpoint": "127.0.0.1:4318",
					},
					"tls": map[string]interface{}{
						"cert_file": "path/to/cert.crt",
						"key_file":  "path/to/key.key",
					},
				}},
			wantErr: &common.MissingKeyError{
				ID:      tt.ID(),
				JsonKey: common.ConfigKey(common.TracesKey, common.TracesCollectedKey, common.OtlpKey),
			},
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
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*otlpreceiver.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, testCase.want.Unmarshal(wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}

func TestMetricsTranslator(t *testing.T) {
	multiConfig := map[string]interface{}{"metrics": map[string]interface{}{
		"metrics_collected": map[string]interface{}{
			"otlp": []any{
				map[string]interface{}{},
				map[string]interface{}{
					"grpc_endpoint": "127.0.0.1:1234",
					"http_endpoint": "127.0.0.1:2345",
				},
			},
		},
	}}

	testCases := map[string]struct {
		input   map[string]interface{}
		index   int
		want    *confmap.Conf
		wantErr error
	}{
		"WithMissingKey": {
			input: map[string]interface{}{"metrics": map[string]interface{}{}},
			index: -1,
			wantErr: &common.MissingKeyError{
				ID:      NewTranslator(WithSignal(pipeline.SignalMetrics)).ID(),
				JsonKey: common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.OtlpKey),
			},
		},
		"WithDefault": {
			input: map[string]interface{}{"metrics": map[string]interface{}{"metrics_collected": map[string]interface{}{"otlp": map[string]interface{}{}}}},
			index: -1,
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
		"WithMultiple_0": {
			input: multiConfig,
			index: 0,
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
		"WithMultiple_1": {
			input: multiConfig,
			index: 1,
			want: confmap.NewFromStringMap(map[string]interface{}{
				"protocols": map[string]interface{}{
					"grpc": map[string]interface{}{
						"endpoint": "127.0.0.1:1234",
					},
					"http": map[string]interface{}{
						"endpoint": "127.0.0.1:2345",
					},
				},
			}),
		},
		"WithCompleteConfig": {
			input: testutil.GetJson(t, filepath.Join("testdata", "metrics", "config.json")),
			index: -1,
			want:  testutil.GetConf(t, filepath.Join("testdata", "metrics", "config.yaml")),
		},
	}
	factory := otlpreceiver.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			tt := NewTranslator(WithSignal(pipeline.SignalMetrics), WithConfigKey(common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.OtlpKey)))
			if testCase.index != -1 {
				tt = NewTranslator(WithSignal(pipeline.SignalMetrics), WithConfigKey(common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.OtlpKey)), common.WithIndex(testCase.index))
			}
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*otlpreceiver.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, testCase.want.Unmarshal(wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}

func TestMetricsEmfTranslator(t *testing.T) {
	multiConfig := map[string]interface{}{"logs": map[string]interface{}{
		"metrics_collected": map[string]interface{}{
			"otlp": []any{
				map[string]interface{}{},
				map[string]interface{}{
					"grpc_endpoint": "127.0.0.1:1234",
					"http_endpoint": "127.0.0.1:2345",
				},
			},
		},
	}}

	testCases := map[string]struct {
		input   map[string]interface{}
		index   int
		want    *confmap.Conf
		wantErr error
	}{
		"WithMissingKey": {
			input: map[string]interface{}{"logs": map[string]interface{}{}},
			index: -1,
			wantErr: &common.MissingKeyError{
				ID:      NewTranslator(WithSignal(pipeline.SignalMetrics)).ID(),
				JsonKey: common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.OtlpKey),
			},
		},
		"WithDefault": {
			input: map[string]interface{}{"logs": map[string]interface{}{"metrics_collected": map[string]interface{}{"otlp": map[string]interface{}{}}}},
			index: -1,
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
		"WithMultiple_0": {
			input: multiConfig,
			index: 0,
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
		"WithMultiple_1": {
			input: multiConfig,
			index: 1,
			want: confmap.NewFromStringMap(map[string]interface{}{
				"protocols": map[string]interface{}{
					"grpc": map[string]interface{}{
						"endpoint": "127.0.0.1:1234",
					},
					"http": map[string]interface{}{
						"endpoint": "127.0.0.1:2345",
					},
				},
			}),
		},
		"WithCompleteConfig": {
			input: testutil.GetJson(t, filepath.Join("testdata", "metrics_emf", "config.json")),
			index: -1,
			want:  testutil.GetConf(t, filepath.Join("testdata", "metrics_emf", "config.yaml")),
		},
	}
	factory := otlpreceiver.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			tt := NewTranslator(
				WithSignal(pipeline.SignalMetrics),
				WithConfigKey(common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.OtlpKey)),
				common.WithIndex(testCase.index),
			)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*otlpreceiver.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, testCase.want.Unmarshal(wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}

func TestTranslateAppSignals(t *testing.T) {
	tt := NewTranslator(common.WithName(common.AppSignals), WithSignal(pipeline.SignalTraces))
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *confmap.Conf
		wantErr error
	}{
		"WithAppSignalsEnabledTraces": {
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{},
					},
				}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"protocols": map[string]interface{}{
					"grpc": map[string]interface{}{
						"endpoint": "0.0.0.0:4315",
					},
					"http": map[string]interface{}{
						"endpoint": "0.0.0.0:4316",
					},
				},
			}),
		},
		"WithAppSignalsEnabledTracesWithTLS": {
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{
							"tls": map[string]interface{}{
								"cert_file": "path/to/cert.crt",
								"key_file":  "path/to/key.key",
							},
						},
					},
				}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"protocols": map[string]interface{}{
					"grpc": map[string]interface{}{
						"endpoint": "0.0.0.0:4315",
						"tls": map[string]interface{}{
							"cert_file": "path/to/cert.crt",
							"key_file":  "path/to/key.key",
						},
					},
					"http": map[string]interface{}{
						"endpoint": "0.0.0.0:4316",
						"tls": map[string]interface{}{
							"cert_file": "path/to/cert.crt",
							"key_file":  "path/to/key.key",
						},
					},
				},
			}),
		},
		"WithAppSignalsFallbackEnabledTraces": {
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"protocols": map[string]interface{}{
					"grpc": map[string]interface{}{
						"endpoint": "0.0.0.0:4315",
					},
					"http": map[string]interface{}{
						"endpoint": "0.0.0.0:4316",
					},
				},
			}),
		},
		"WithAppSignalsFallbackEnabledTracesWithTLS": {
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{
							"tls": map[string]interface{}{
								"cert_file": "path/to/cert.crt",
								"key_file":  "path/to/key.key",
							},
						},
					},
				}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"protocols": map[string]interface{}{
					"grpc": map[string]interface{}{
						"endpoint": "0.0.0.0:4315",
						"tls": map[string]interface{}{
							"cert_file": "path/to/cert.crt",
							"key_file":  "path/to/key.key",
						},
					},
					"http": map[string]interface{}{
						"endpoint": "0.0.0.0:4316",
						"tls": map[string]interface{}{
							"cert_file": "path/to/cert.crt",
							"key_file":  "path/to/key.key",
						},
					},
				},
			}),
		},
	}
	factory := otlpreceiver.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*otlpreceiver.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, testCase.want.Unmarshal(wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}

func TestTranslateJMX(t *testing.T) {
	tt := NewTranslator(common.WithName(common.PipelineNameJmx))
	got, err := tt.Translate(nil)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	gotCfg, ok := got.(*otlpreceiver.Config)
	require.True(t, ok)
	assert.Nil(t, gotCfg.GRPC)
	assert.NotNil(t, gotCfg.HTTP)
	assert.Equal(t, "0.0.0.0:4314", gotCfg.HTTP.ServerConfig.Endpoint)
}
