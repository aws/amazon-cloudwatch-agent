// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcedetection

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslate(t *testing.T) {
	tt := NewTranslator(WithDataType(component.DataTypeTraces))
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *confmap.Conf
		wantErr error
	}{
		"WithAppSignalsEnabled": {
			input: map[string]interface{}{
				"traces": map[string]interface{}{
					"traces_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				}},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"detectors": []interface{}{
					"eks",
					"env",
					"ec2",
				},
				"timeout":  "2s",
				"override": true,
				"ec2": map[string]interface{}{
					"tags": []interface{}{"^kubernetes.io/cluster/.*$", "^aws:autoscaling:groupName"},
				},
			}),
		},
	}
	factory := resourcedetectionprocessor.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*resourcedetectionprocessor.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, component.UnmarshalConfig(testCase.want, wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
