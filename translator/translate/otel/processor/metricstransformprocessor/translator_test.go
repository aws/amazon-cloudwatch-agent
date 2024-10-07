// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metricstransformprocessor

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		translator common.Translator[component.Config]
		input      map[string]interface{}
		want       map[string]interface{}
		wantErr    error
	}{
		"DefaultMetricsSection": {
			translator: NewTranslatorWithName("test"),
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			want: map[string]interface{}{
				"transforms": []map[string]interface{}{
					{
						"include":                   "apiserver_request_total",
						"match_type":                "regexp",
						"experimental_match_labels": map[string]string{"code": "^5.*"},
						"action":                    "insert",
						"new_name":                  "apiserver_request_total_5xx",
					},
				},
			},
		},
		"JMXMetricsSection": {
			translator: NewTranslatorWithName("jmx"),
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			want: map[string]interface{}{
				"transforms": []map[string]interface{}{
					{
						"include":                   "apiserver_request_total",
						"match_type":                "regexp",
						"experimental_match_labels": map[string]string{"code": "^5.*"},
						"action":                    "insert",
						"new_name":                  "apiserver_request_total_5xx",
					},
					{
						"include": "tomcat.sessions",
						"action":  "update",
						"operations": []map[string]interface{}{
							{
								"action":           "aggregate_labels",
								"aggregation_type": "sum",
							},
							{
								"action": "delete_label_value",
								"label":  "context",
							},
						},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tc.input)
			got, err := tc.translator.Translate(conf)
			require.Equal(t, tc.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*metricstransformprocessor.Config)
				require.True(t, ok)
				require.Equal(t, tc.want["transforms"].([]map[string]interface{})[0]["include"], gotCfg.Transforms[0].MetricIncludeFilter.Include)
				require.Equal(t, tc.want["transforms"].([]map[string]interface{})[0]["match_type"], fmt.Sprint(gotCfg.Transforms[0].MetricIncludeFilter.MatchType))
				require.Equal(t, tc.want["transforms"].([]map[string]interface{})[0]["experimental_match_labels"], gotCfg.Transforms[0].MetricIncludeFilter.MatchLabels)
				require.Equal(t, tc.want["transforms"].([]map[string]interface{})[0]["action"], fmt.Sprint(gotCfg.Transforms[0].Action))
				require.Equal(t, tc.want["transforms"].([]map[string]interface{})[0]["new_name"], gotCfg.Transforms[0].NewName)
				if name == "JMXMetricsSection" {
					require.Equal(t, tc.want["transforms"].([]map[string]interface{})[1]["include"], gotCfg.Transforms[1].MetricIncludeFilter.Include)
					require.Equal(t, tc.want["transforms"].([]map[string]interface{})[1]["action"], fmt.Sprint(gotCfg.Transforms[1].Action))
					require.Equal(t, tc.want["transforms"].([]map[string]interface{})[1]["operations"].([]map[string]interface{})[0]["action"], fmt.Sprint(gotCfg.Transforms[1].Operations[0].Action))
					require.Equal(t, tc.want["transforms"].([]map[string]interface{})[1]["operations"].([]map[string]interface{})[0]["aggregation_type"], fmt.Sprint(gotCfg.Transforms[1].Operations[0].AggregationType))
					require.Equal(t, tc.want["transforms"].([]map[string]interface{})[1]["operations"].([]map[string]interface{})[1]["action"], fmt.Sprint(gotCfg.Transforms[1].Operations[1].Action))
					require.Equal(t, tc.want["transforms"].([]map[string]interface{})[1]["operations"].([]map[string]interface{})[1]["label"], fmt.Sprint(gotCfg.Transforms[1].Operations[1].Label))
				}
			}
		})
	}
}
