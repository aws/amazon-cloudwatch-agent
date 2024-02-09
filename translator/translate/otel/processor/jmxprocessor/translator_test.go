// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxprocessor

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
	require.EqualValues(t, "filter", jmxTranslator.ID().String())
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
							"jvm": map[string]interface{}{},
						},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"filter/1": map[string]interface{}{
					"metrics": map[string]interface{}{
						"include": map[string]interface{}{
							"match_type": "regexp",
							"metric_names": []interface{}{
								"jvm.memory.heap.init",
								"jvm.memory.heap.used",
								"jvm.memory.nonheap.init",
								"jvm.memory.nonheap.used",
								"jvm.threads.count",
								"tomcat*",
							},
						},
					},
				},
			}),
		},
		"ConfigWithJmxTargetNoMetricNameNoTarget": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"jmx": map[string]interface{}{},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"filter/1": map[string]interface{}{
					"metrics": map[string]interface{}{
						"include": map[string]interface{}{
							"match_type": "regexp",
							"metric_names": []interface{}{
								"*",
							},
						},
					},
				},
			}),
		},
		"ConfigWithJmxTargetWithMetricName": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"jmx": map[string]interface{}{
							"jvm": []string{
								"jvm.memory.heap.init"},
						},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"filter/1": map[string]interface{}{
					"metrics": map[string]interface{}{
						"include": map[string]interface{}{
							"match_type": "regexp",
							"metric_names": []interface{}{
								"jvm.memory.heap.init",
							},
						},
					},
				},
			}),
		},
		"ConfigWithMultipleJmxTargetWithNoMetricName": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"jmx": map[string]interface{}{
							"jvm":    map[string]interface{}{},
							"hadoop": map[string]interface{}{},
						},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"filter/1": map[string]interface{}{
					"metrics": map[string]interface{}{
						"include": map[string]interface{}{
							"match_type": "regexp",
							"metric_names": []interface{}{
								"jvm*",
								"hadoop*",
							},
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
							"jvm": []string{
								"jvm.memory.heap.init"},
							"hadoop": map[string]interface{}{},
						},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"filter/1": map[string]interface{}{
					"metrics": map[string]interface{}{
						"include": map[string]interface{}{
							"match_type": "regexp",
							"metric_names": []interface{}{
								"jvm.memory.heap.init",
								"hadoop*",
							},
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
							"jvm": []string{
								"jvm.memory.heap.init",
								"jvm.memory.heap.used",
								"jvm.memory.nonheap.init",
								"jvm.memory.nonheap.used",
								"jvm.threads.count"},
							"hadoop": map[string]interface{}{},
							"tomcat": []string{
								"tomcat.sessions",
								"tomcat.request_count",
								"tomcat.traffic",
								"tomcat.errors"},
						},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"filter/1": map[string]interface{}{
					"metrics": map[string]interface{}{
						"include": map[string]interface{}{
							"match_type": "regexp",
							"metric_names": []interface{}{
								"jvm.memory.heap.init",
								"jvm.memory.heap.used",
								"jvm.memory.nonheap.init",
								"jvm.memory.nonheap.used",
								"jvm.threads.count",
								"hadoop*",
								"tomcat.sessions",
								"tomcat.request_count",
								"tomcat.traffic",
								"tomcat.errors",
							},
						},
					},
				},
			}),
		},
		"WithCompleteConfig": {
			input: testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "config.yaml")),
		},
	}

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
