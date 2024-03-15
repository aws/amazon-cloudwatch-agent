// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxfilterprocessor

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	jmxTranslator := NewTranslator()

	require.EqualValues(t, "filter/jmx", jmxTranslator.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *confmap.Conf
		wantErr error
	}{
		"ConfigWithNoJmxSet": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{},
					},
				},
			},
			wantErr: &common.MissingKeyError{ID: jmxTranslator.ID(), JsonKey: fmt.Sprint(jmxKey)},
		},
		"ConfigWithJmxTargetNoMetricName": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"jmx": map[string]interface{}{
							"jvm": []interface{}{},
						},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"include": map[string]interface{}{
						"match_type":   "regexp",
						"metric_names": []interface{}{"jvm\\..*"},
					},
				},
			}),
		},

		"ConfigWithJmxTargetWithMetricName": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"jmx": map[string]interface{}{
							"jvm": []interface{}{
								"jvm.memory.heap.init",
							},
						},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"include": map[string]interface{}{
						"match_type":   "regexp",
						"metric_names": []interface{}{"jvm.memory.heap.init"},
					},
				},
			}),
		},
		"ConfigWithMultipleJmxTargetWithNoMetricName": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"jmx": map[string]interface{}{
							"jvm":    []interface{}{},
							"hadoop": []interface{}{},
						},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"include": map[string]interface{}{
						"match_type": "regexp",
						"metric_names": []interface{}{
							"hadoop\\..*",
							"jvm\\..*",
						},
					},
				},
			}),
		},
		"ConfigWithMultipleJmxTargetAlternating": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"jmx": map[string]interface{}{
							"jvm": []interface{}{
								"jvm.memory.heap.init"},
							"hadoop": []interface{}{},
						},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"include": map[string]interface{}{
						"match_type": "regexp",
						"metric_names": []interface{}{
							"hadoop\\..*",
							"jvm.memory.heap.init",
						},
					},
				},
			}),
		},
		"ConfigWithMultiple": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"jmx": map[string]interface{}{
							"jvm": []interface{}{
								"jvm.memory.heap.init",
								"jvm.threads.count"},
							"hadoop": []interface{}{},
							"tomcat": []interface{}{
								"tomcat.sessions",
								"tomcat.errors"},
						},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"include": map[string]interface{}{
						"match_type": "regexp",
						"metric_names": []interface{}{
							"hadoop\\..*",
							"jvm.memory.heap.init",
							"jvm.threads.count",
							"tomcat.sessions",
							"tomcat.errors"},
					},
				},
			}),
		},
		"WithCompleteConfig": {
			input: testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "config.yaml")),
		},
	}
	factory := filterprocessor.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := jmxTranslator.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*filterprocessor.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, component.UnmarshalConfig(testCase.want, wantCfg))
				require.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
