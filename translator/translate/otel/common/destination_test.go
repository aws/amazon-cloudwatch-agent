// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestGetMetricsDestinations(t *testing.T) {
	testCases := map[string]struct {
		input map[string]any
		want  []string
	}{
		"WithNoMetrics": {
			input: map[string]any{
				"logs": map[string]any{},
			},
			want: []string{DefaultDestination},
		},
		"WithMetrics/Default": {
			input: map[string]any{
				"metrics": map[string]any{},
			},
			want: []string{DefaultDestination},
		},
		"WithMetrics/AMP": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"amp": map[string]any{},
					},
				},
			},
			want: []string{AMPKey},
		},
		"WithMetrics/CloudWatch": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"cloudwatch": map[string]any{},
					},
				},
			},
			want: []string{CloudWatchKey},
		},
		"WithMetrics/CloudWatch&AMP": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"cloudwatch": map[string]any{},
						"amp":        map[string]any{},
					},
				},
			},
			want: []string{CloudWatchKey, AMPKey},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got := GetMetricsDestinations(conf)
			assert.Equal(t, testCase.want, got)
		})
	}
}
