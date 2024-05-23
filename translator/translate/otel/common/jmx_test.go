// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestGetJmxMap(t *testing.T) {
	testCases := map[string]struct {
		input map[string]any
		index int
		want  map[string]any
	}{
		"WithObject": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": map[string]any{
							"endpoint": "test",
						},
					},
				},
			},
			want: map[string]any{
				"endpoint": "test",
			},
		},
		"WithArray/InvalidIndex": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": []any{
							map[string]any{
								"endpoint": "test",
							},
						},
					},
				},
			},
			index: -1,
			want:  nil,
		},
		"WithArray/IndexOutOfBounds": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": []any{
							map[string]any{
								"endpoint": "test",
							},
						},
					},
				},
			},
			index: 1,
			want:  nil,
		},
		"WithArray/Valid": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"jmx": []any{
							map[string]any{
								"endpoint": "test",
							},
						},
					},
				},
			},
			index: 0,
			want: map[string]any{
				"endpoint": "test",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got := GetJmxMap(conf, testCase.index)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestGetMeasurements(t *testing.T) {
	testCases := map[string]struct {
		input map[string]any
		want  []string
	}{
		"WithEmpty": {
			input: map[string]any{
				"measurement": []any{},
			},
			want: nil,
		},
		"WithInvalid": {
			input: map[string]any{
				"measurement": []any{1, 2},
			},
			want: nil,
		},
		"WithValid": {
			input: map[string]any{
				"measurement": []any{"1", "2"},
			},
			want: []string{"1", "2"},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, testCase.want, GetMeasurements(testCase.input))
		})
	}
}
