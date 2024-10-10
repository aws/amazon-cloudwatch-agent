// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourceprocessor

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		name        string
		index       int
		isContainer bool
		input       map[string]any
		wantID      string
		want        *confmap.Conf
		wantErr     error
	}{
		"WithoutJMX": {
			name: common.PipelineNameJmx,
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"cpu": map[string]any{},
					},
				},
			},
			index:   -1,
			wantID:  "resource/jmx",
			wantErr: &common.MissingKeyError{ID: component.MustNewIDWithName("resource", "jmx"), JsonKey: common.JmxConfigKey},
		},
		"WithJMX": {
			name: common.PipelineNameJmx,
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": map[string]any{
							"append_dimensions": map[string]any{
								"unused": "by resource processor",
							},
						},
					},
				},
			},
			index:  -1,
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
		"WithJMX/EKS/NoAppendDimensions": {
			name: common.PipelineNameJmx,
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": map[string]any{},
					},
				},
			},
			index:       -1,
			isContainer: true,
			wantID:      "resource/jmx",
			wantErr: &common.MissingKeyError{
				ID:      component.MustNewIDWithName("resource", "jmx"),
				JsonKey: "metrics::metrics_collected::jmx::append_dimensions",
			},
		},
		"WithJMX/EKS/AppendDimensions": {
			name: common.PipelineNameJmx,
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": map[string]any{
							"append_dimensions": map[string]any{
								"k1": "v1",
							},
						},
					},
				},
			},
			index:       -1,
			isContainer: true,
			wantID:      "resource/jmx",
			want: confmap.NewFromStringMap(map[string]any{
				"attributes": []any{
					map[string]any{
						"action": "upsert",
						"key":    "k1",
						"value":  "v1",
					},
				},
			}),
		},
		"WithJMX/Array/EKS/InvalidAppendDimensions": {
			name: common.PipelineNameJmx,
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": []any{
							map[string]any{
								"append_dimensions": []any{
									"invalid",
								},
							},
						},
					},
				},
			},
			index:       0,
			isContainer: true,
			wantID:      "resource/jmx/0",
			wantErr: &common.MissingKeyError{
				ID:      component.MustNewIDWithName("resource", "jmx/0"),
				JsonKey: "metrics::metrics_collected::jmx[0]::append_dimensions",
			},
		},
		"WithJMX/Array/EKS/AppendDimensions": {
			name: common.PipelineNameJmx,
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": []any{
							map[string]any{
								"append_dimensions": map[string]any{
									"k1": "v1",
								},
							},
							map[string]any{
								"append_dimensions": map[string]any{
									"k2": "v2",
								},
							},
						},
					},
				},
			},
			index:       1,
			isContainer: true,
			wantID:      "resource/jmx/1",
			want: confmap.NewFromStringMap(map[string]any{
				"attributes": []any{
					map[string]any{
						"action": "upsert",
						"key":    "k2",
						"value":  "v2",
					},
				},
			}),
		},
	}
	factory := resourceprocessor.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			context.CurrentContext().SetRunInContainer(testCase.isContainer)
			tt := NewTranslator(common.WithName(testCase.name), common.WithIndex(testCase.index))
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.EqualValues(t, testCase.wantID, tt.ID().String())
			assert.Equal(t, testCase.wantErr, err)
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

func TestContainerInsightsJmx(t *testing.T) {
	transl := NewTranslator(common.WithName(common.PipelineNameContainerInsightsJmx)).(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*resourceprocessor.Config)
	c := testutil.GetConf(t, filepath.Join("testdata", "config.yaml"))
	require.NoError(t, c.Unmarshal(&expectedCfg))

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("testdata", "config.json")))
	translatedCfg, err := transl.Translate(conf)
	assert.NoError(t, err)
	actualCfg, ok := translatedCfg.(*resourceprocessor.Config)
	assert.True(t, ok)
	assert.Equal(t, len(actualCfg.AttributesActions), len(expectedCfg.AttributesActions))

}
