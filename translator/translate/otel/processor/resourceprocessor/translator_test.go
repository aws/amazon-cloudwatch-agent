// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourceprocessor

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		name    string
		input   map[string]any
		wantID  string
		want    *confmap.Conf
		wantErr error
	}{
		"ConfigWithNoJmxSet": {
			name: common.PipelineNameJmx,
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"cpu": map[string]any{},
					},
				},
			},
			wantID:  "resource/jmx",
			wantErr: &common.MissingKeyError{ID: component.MustNewIDWithName("resource", "jmx"), JsonKey: common.JmxConfigKey},
		},
		"ConfigWithJmx": {
			name: common.PipelineNameJmx,
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": map[string]any{},
					},
				},
			},
			wantID: "resource/jmx",
			want: confmap.NewFromStringMap(map[string]any{
				"attributes": []any{
					map[string]any{
						"action":  "delete",
						"pattern": "telemetry.sdk.*",
					},
					map[string]any{
						"action": "delete",
						"key":    "service.name",
						"value":  "unknown_service:java",
					},
				},
			}),
		},
	}
	factory := resourceprocessor.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator(WithName(testCase.name))
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.EqualValues(t, testCase.wantID, tt.ID().String())
			assert.Equal(t, err, testCase.wantErr)
			if err == nil {
				assert.NotNil(t, got)
				gotCfg, ok := got.(*resourceprocessor.Config)
				assert.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				assert.NoError(t, testCase.want.Unmarshal(wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
