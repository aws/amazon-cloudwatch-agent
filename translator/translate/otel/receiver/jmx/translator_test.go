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
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslator(WithDataType(component.DataTypeMetrics))
	assert.EqualValues(t, "jmx/metrics", tt.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *confmap.Conf
		wantErr error
	}{
		"WithMissingKey": {
			input: map[string]interface{}{"logs": map[string]interface{}{}},
			wantErr: &common.MissingKeyError{
				ID:      tt.ID(),
				JsonKey: common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.JmxKey),
			},
		},
		"WithDefault": {
			input: map[string]interface{}{"metrics": map[string]interface{}{"metrics_collected": map[string]interface{}{"jmx": nil}}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"jar_path":            defaultJMXJarPath,
				"target_system":       defaultTargetSystem,
				"collection_interval": "10s",
				"otlp": map[string]interface{}{
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
			t.Log(name)
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
