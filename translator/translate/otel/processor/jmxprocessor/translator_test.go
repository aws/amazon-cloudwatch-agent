// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxprocessor

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslator(t *testing.T) {
	jmxTranslator := NewTranslator()
	require.EqualValues(t, "filter", jmxTranslator.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *filterprocessor.Config
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
			want: &filterprocessor.Config{
				Include: filterprocessor.MetricFilters{Include: []string{"jvm*"}},
			},
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
			want: &filterprocessor.Config{
				Include: filterprocessor.MetricFilters{Include: []string{"jvm.memory.heap.init"}},
			},
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
			want: &filterprocessor.Config{
				Include: filterprocessor.MetricFilters{Include: []string{"jvm*", "hadoop*"}},
			},
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
			want: &filterprocessor.Config{
				Include: filterprocessor.MetricFilters{Include: []string{"jvm.memory.heap.init", "hadoop*"}},
			},
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
			want: &filterprocessor.Config{
				Include: filterprocessor.MetricFilters{Include: []string{"jvm.memory.heap.init", "jvm.memory.heap.used",
					"jvm.memory.nonheap.init", "jvm.memory.nonheap.used", "jvm.threads.count", "hadoop*", "tomcat.sessions",
					"tomcat.request_count", "tomcat.traffic", "tomcat.errors"}},
			},
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
				require.Equal(t, testCase.want.Include.Metrics, gotCfg.Include.Metrics)
]			}
		})
	}
}
