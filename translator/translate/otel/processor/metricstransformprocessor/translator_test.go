// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metricstransformprocessor

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		translator common.Translator[component.Config]
		input      map[string]interface{}
		want       *confmap.Conf
		wantErr    error
	}{
		"DefaultMetricsSection": {
			translator: NewTranslatorWithName("containerinsights"),
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"accelerated_compute_metrics": false,
						},
					},
				},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"transforms": []map[string]interface{}{
					{
						"include":                   "apiserver_request_total",
						"match_type":                "regexp",
						"experimental_match_labels": map[string]string{"code": "^5.*"},
						"action":                    "insert",
						"new_name":                  "apiserver_request_total_5xx",
					},
				},
			}),
		},
		"JMXMetricsSection": {
			translator: NewTranslatorWithName("jmx"),
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			want: confmap.NewFromStringMap(map[string]interface{}{
				"transforms": []map[string]interface{}{
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
			}),
		},
		"UnknownProcessorName": {
			translator: NewTranslatorWithName("unknown"),
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			wantErr: fmt.Errorf("no transform rules for unknown"),
		},
	}
	factory := metricstransformprocessor.NewFactory()
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tc.input)
			got, err := tc.translator.Translate(conf)
			require.Equal(t, tc.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*metricstransformprocessor.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, tc.want.Unmarshal(&wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
