// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmx

import (
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
	tt := NewTranslator()
	assert.EqualValues(t, "jmx", tt.ID().String())
	testCases := map[string]struct {
		input   map[string]any
		want    *confmap.Conf
		wantErr error
	}{
		"WithMissingKey": {
			input: map[string]any{"logs": map[string]any{}},
			wantErr: &common.MissingKeyError{
				ID:      tt.ID(),
				JsonKey: common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.JmxKey),
			},
		},
		"WithDefault": {
			input: map[string]any{"metrics": map[string]any{"metrics_collected": map[string]any{"jmx": nil}}},
			want: confmap.NewFromStringMap(map[string]any{
				"jar_path":            paths.JMXJarPath,
				"target_system":       defaultTargetSystem,
				"collection_interval": "10s",
				"otlp": map[string]any{
					"endpoint": "127.0.0.1:3000",
					"timeout":  "5s",
				},
			}),
		},
		"WithCompleteConfig": {
			input: testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "config.yaml")),
		},
	}
	factory := jmxreceiver.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*jmxreceiver.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, component.UnmarshalConfig(testCase.want, wantCfg))
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
				"endpoint":      "my_jmx_host:12345",
				"password_file": "/path/to/password_file",
			},
			wantErr: &missingFieldsError{
				fields: []string{
					usernameKey,
					keystorePathKey,
					keystoreTypeKey,
					truststorePathKey,
					truststoreTypeKey,
				},
			},
		},
		"WithOptOut": {
			jmxSectionInput: map[string]any{
				"endpoint": "my_jmx_host:12345",
				"insecure": true,
			},
		},
		"WithAllSet": {
			jmxSectionInput: map[string]any{
				"endpoint":        "my_jmx_host:12345",
				"username":        "myusername",
				"password_file":   "/path/to/password_file",
				"keystore_path":   "/path/to/keystore",
				"keystore_type":   "PKCS",
				"truststore_path": "/path/to/truststore",
				"truststore_type": "PKCS12",
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
