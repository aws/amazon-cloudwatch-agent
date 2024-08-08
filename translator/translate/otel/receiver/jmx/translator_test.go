// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	factory := jmxreceiver.NewFactory()
	hostname, _ := os.Hostname()
	testCases := map[string]struct {
		input   map[string]any
		index   int
		wantID  string
		want    *confmap.Conf
		wantErr error
	}{
		"WithMissingKey": {
			input:  map[string]any{"logs": map[string]any{}},
			index:  -1,
			wantID: "jmx",
			wantErr: &common.MissingKeyError{
				ID:      component.NewID(factory.Type()),
				JsonKey: common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.JmxKey),
			},
		},
		"WithMissingEndpoint": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": map[string]any{},
					},
				},
			},
			index:   -1,
			wantID:  "jmx",
			wantErr: errNoEndpoint,
		},
		"WithMissingTargetSystems": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": map[string]any{
							"endpoint": "localhost:8080",
						},
					},
				},
			},
			index:   -1,
			wantID:  "jmx",
			wantErr: errNoTargetSystems,
		},
		"WithValid": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": []any{
							map[string]any{
								"endpoint": "localhost:8080",
								"tomcat": map[string]any{
									"measurement": []any{
										"tomcat.sessions",
									},
								},
							},
						},
					},
				},
			},
			index:  0,
			wantID: "jmx/0",
			want: confmap.NewFromStringMap(map[string]any{
				"endpoint":            "localhost:8080",
				"target_system":       "tomcat",
				"collection_interval": "60s",
				"otlp": map[string]any{
					"endpoint": "0.0.0.0:0",
					"timeout":  "5s",
				},
				"resource_attributes": map[string]string{
					attributeHost: hostname,
				},
			}),
		},
		"WithCompleteConfig": {
			input:  testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			index:  -1,
			wantID: "jmx",
			want:   testutil.GetConf(t, filepath.Join("testdata", "config.yaml")),
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator(WithIndex(testCase.index))
			assert.EqualValues(t, testCase.wantID, tt.ID().String())
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*jmxreceiver.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig().(*jmxreceiver.Config)
				require.NoError(t, testCase.want.Unmarshal(wantCfg))
				wantCfg.JARPath = paths.JMXJarPath
				if wantCfg.ResourceAttributes != nil && wantCfg.ResourceAttributes[attributeHost] == attributeHost {
					wantCfg.ResourceAttributes[attributeHost] = hostname
				}
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}

func TestValidateAuth(t *testing.T) {
	tt := NewTranslator()
	testCases := map[string]struct {
		jmxSectionInput map[string]any
		wantErr         error
	}{
		"WithMissingFields": {
			jmxSectionInput: map[string]any{
				"endpoint": "my_jmx_host:12345",
				"jvm": map[string]any{
					"measurement": []any{
						"jvm.memory.heap.init",
					},
				},
				"password_file": "/path/to/password_file",
			},
			wantErr: &missingFieldsError{
				fields: []string{
					keystorePathKey,
					keystoreTypeKey,
					truststorePathKey,
					truststoreTypeKey,
					usernameKey,
				},
			},
		},
		"WithOptOut": {
			jmxSectionInput: map[string]any{
				"endpoint": "my_jmx_host:12345",
				"jvm": map[string]any{
					"measurement": []any{
						"jvm.memory.heap.init",
					},
				},
				"insecure": true,
			},
		},
		"WithAllSet": {
			jmxSectionInput: map[string]any{
				"endpoint": "my_jmx_host:12345",
				"jvm": map[string]any{
					"measurement": []any{
						"jvm.memory.heap.init",
					},
				},
				"username":             "myusername",
				"password_file":        "/path/to/password_file",
				"keystore_path":        "/path/to/keystore",
				"keystore_type":        "PKCS",
				"truststore_path":      "/path/to/truststore",
				"truststore_type":      "PKCS12",
				"registry_ssl_enabled": true,
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": testCase.jmxSectionInput,
					},
				},
			})
			_, err := tt.Translate(conf)
			if testCase.wantErr != nil {
				assert.ErrorContains(t, err, testCase.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
