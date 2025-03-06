// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmx

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
		"WithSingle": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": map[string]any{},
					},
				},
			},
			want: []pipeline.ID{
				pipeline.MustNewIDWithName("metrics", "jmx"),
			},
		},
		"WithSingle/Destinations": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_destinations": map[string]any{
						"amp": map[string]any{
							"workspace_id": "ws-12345",
						},
					},
					"metrics_collected": map[string]any{
						"jmx": map[string]any{},
					},
				},
			},
			want: []pipeline.ID{
				pipeline.MustNewIDWithName("metrics", "jmx/amp"),
			},
		},
		"WithMultiple": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": []any{
							map[string]any{},
							map[string]any{},
						},
					},
				},
			},
			want: []pipeline.ID{
				pipeline.MustNewIDWithName("metrics", "jmx/0"),
				pipeline.MustNewIDWithName("metrics", "jmx/1"),
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
						"jmx": []any{
							map[string]any{},
							map[string]any{},
						},
					},
				},
			},
			want: []pipeline.ID{
				pipeline.MustNewIDWithName("metrics", "jmx/cloudwatch/0"),
				pipeline.MustNewIDWithName("metrics", "jmx/amp/0"),
				pipeline.MustNewIDWithName("metrics", "jmx/cloudwatch/1"),
				pipeline.MustNewIDWithName("metrics", "jmx/amp/1"),
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
