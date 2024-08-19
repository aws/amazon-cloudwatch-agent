// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	_ "github.com/aws/amazon-cloudwatch-agent/translator/registerrules"
)

func TestTranslators(t *testing.T) {
	testCases := map[string]struct {
		input map[string]any
		want  []component.ID
	}{
		"WithEmpty": {
			input: map[string]any{},
			want:  []component.ID{},
		},
		"WithSingle": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": map[string]any{},
					},
				},
			},
			want: []component.ID{
				component.MustNewIDWithName("metrics", "jmx"),
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
			want: []component.ID{
				component.MustNewIDWithName("metrics", "jmx/0"),
				component.MustNewIDWithName("metrics", "jmx/1"),
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
