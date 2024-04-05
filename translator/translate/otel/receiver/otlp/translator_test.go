// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
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
	tt := NewTranslator(WithDataType(component.DataTypeTraces))
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
				require.NoError(t, component.UnmarshalConfig(testCase.want, wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}

func TestTranslateAppSignals(t *testing.T) {
	tt := NewTranslatorWithName(common.AppSignals, WithDataType(component.DataTypeTraces))
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
				require.NoError(t, component.UnmarshalConfig(testCase.want, wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
