// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
)

func TestMergeWithStatusCodes(t *testing.T) {
	// Initial stats with some status codes
	stats := &Stats{
		StatusCodes: map[string][5]int{
			"operation1": {1, 2, 3, 4, 5},
		},
	}

	// Merge with new stats containing additional status codes
	stats.Merge(Stats{
		StatusCodes: map[string][5]int{
			"operation1": {2, 3, 4, 5, 6}, // Existing operation with new values
			"operation2": {0, 1, 2, 3, 4}, // New operation
		},
	})

	// Assert merged values
	assert.Equal(t, [5]int{3, 5, 7, 9, 11}, stats.StatusCodes["operation1"]) // Values should sum
	assert.Equal(t, [5]int{0, 1, 2, 3, 4}, stats.StatusCodes["operation2"])  // New operation added

	// Merge with empty StatusCodes map
	stats.Merge(Stats{
		StatusCodes: nil,
	})

	// Assert that StatusCodes remains unchanged
	assert.Equal(t, [5]int{3, 5, 7, 9, 11}, stats.StatusCodes["operation1"])
	assert.Equal(t, [5]int{0, 1, 2, 3, 4}, stats.StatusCodes["operation2"])
}

func TestMarshalWithStatusCodes(t *testing.T) {
	testCases := map[string]struct {
		stats *Stats
		want  string
	}{
		"WithEmptyStatusCodes": {
			stats: &Stats{
				StatusCodes: map[string][5]int{},
			},
			want: "",
		},
		"WithStatusCodes": {
			stats: &Stats{
				StatusCodes: map[string][5]int{
					"operation1": {1, 2, 3, 4, 5},
					"operation2": {0, 1, 2, 3, 4},
				},
			},
			want: `"codes":{"operation1":[1,2,3,4,5],"operation2":[0,1,2,3,4]}`,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := testCase.stats.Marshal()
			assert.NoError(t, err)
			assert.Contains(t, got, testCase.want)
		})
	}
}

func TestMergeFullWithStatusCodes(t *testing.T) {
	stats := &Stats{
		CpuPercent:  aws.Float64(1.0),
		StatusCodes: map[string][5]int{"operation1": {1, 0, 0, 0, 0}},
	}
	stats.Merge(Stats{
		CpuPercent:  aws.Float64(2.0),
		StatusCodes: map[string][5]int{"operation1": {0, 1, 0, 0, 0}, "operation2": {1, 1, 1, 1, 1}},
	})

	assert.Equal(t, 2.0, *stats.CpuPercent)
	assert.Equal(t, [5]int{1, 1, 0, 0, 0}, stats.StatusCodes["operation1"])
	assert.Equal(t, [5]int{1, 1, 1, 1, 1}, stats.StatusCodes["operation2"])
}
