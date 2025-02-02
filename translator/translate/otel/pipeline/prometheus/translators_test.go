// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	_ "github.com/aws/amazon-cloudwatch-agent/translator/registerrules"
)

func TestTranslators(t *testing.T) {

	testCases := map[string]struct {
		input map[string]any
		want  []pipeline.ID
	}{
		"WithEmpty": {
			input: map[string]any{},
			want:  []pipeline.ID{},
		},
		"WithMetricsWithoutDestinations": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{},
					},
				},
			},
			want: []pipeline.ID{
				pipeline.MustNewIDWithName("metrics", "prometheus/amp"),
			},
		},
		"WithLogsWithoutDestinations": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{},
					},
				},
			},
			want: []pipeline.ID{
				pipeline.MustNewIDWithName("metrics", "prometheus/cloudwatchlogs"),
			},
		},
		"WithMetricsWithCloudWatchDestination": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"cloudwatch": map[string]any{},
					},
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{},
					},
				},
			},
			want: []pipeline.ID{
				pipeline.MustNewIDWithName("metrics", "prometheus/amp"),
			},
		},
		"WithMetricsWithAMP": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"amp": map[string]any{
							"workspace_id": "ws-12345",
						},
					},
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{},
					},
				},
			},
			want: []pipeline.ID{
				pipeline.MustNewIDWithName("metrics", "prometheus/amp"),
			},
		},
		"WithLogsWithCloudWatch": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"cloudwatch": map[string]any{},
					},
				},
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{},
					},
				},
			},
			want: []pipeline.ID{
				pipeline.MustNewIDWithName("metrics", "prometheus/cloudwatchlogs"),
			},
		},
		"WithMultiple/Destinations": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"cloudwatch": map[string]any{},
						"amp": map[string]any{
							"workspace_id": "ws-12345",
						},
					},
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{},
					},
				},
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"prometheus": map[string]any{},
					},
				},
			},
			want: []pipeline.ID{
				pipeline.MustNewIDWithName("metrics", "prometheus/amp"),
				pipeline.MustNewIDWithName("metrics", "prometheus/cloudwatchlogs"),
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got := NewTranslators(conf)
			if testCase.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, len(testCase.want), got.Len())
				for _, id := range testCase.want {
					_, ok := got.Get(id)
					assert.True(t, ok)
				}
			}
		})
	}
}
