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
				"transforms": map[string]interface{}{
					"include":                   "apiserver_request_total",
					"match_type":                "regexp",
					"experimental_match_labels": map[string]string{"code": "^5.*"},
					"action":                    "insert",
					"new_name":                  "apiserver_request_total_5xx",
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
				require.Equal(t, tc.want["transforms"].(map[string]interface{})["include"], gotCfg.Transforms[0].MetricIncludeFilter.Include)
				require.Equal(t, tc.want["transforms"].(map[string]interface{})["match_type"], fmt.Sprint(gotCfg.Transforms[0].MetricIncludeFilter.MatchType))
				require.Equal(t, tc.want["transforms"].(map[string]interface{})["experimental_match_labels"], gotCfg.Transforms[0].MetricIncludeFilter.MatchLabels)
				require.Equal(t, tc.want["transforms"].(map[string]interface{})["action"], fmt.Sprint(gotCfg.Transforms[0].Action))
				require.Equal(t, tc.want["transforms"].(map[string]interface{})["new_name"], gotCfg.Transforms[0].NewName)
			}
		})
	}
}
