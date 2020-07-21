// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsmmetrics

import (
	"reflect"
	"testing"
)

func TestCountSample(t *testing.T) {
	cases := []struct {
		testName             string
		sampleValue          string
		initialDistribution  FrequencyMetric
		expectedDistribution FrequencyMetric
	}{
		{
			testName:            "empty distribution",
			sampleValue:         "bar",
			initialDistribution: NewFrequencyMetric("foo"),
			expectedDistribution: FrequencyMetric{
				Name: "foo",
				Frequencies: map[string]int64{
					"bar": 1,
				},
			},
		},
		{
			testName:    "non-empty distribution with unseen sample",
			sampleValue: "baz",
			initialDistribution: FrequencyMetric{
				Name: "bar",
				Frequencies: map[string]int64{
					"fu": 1,
				},
			},
			expectedDistribution: FrequencyMetric{
				Name: "bar",
				Frequencies: map[string]int64{
					"fu":  1,
					"baz": 1,
				},
			},
		},
		{
			testName:    "non-empty distribution with existing sample",
			sampleValue: "baz",
			initialDistribution: FrequencyMetric{
				Name: "bar",
				Frequencies: map[string]int64{
					"baz": 5,
				},
			},
			expectedDistribution: FrequencyMetric{
				Name: "bar",
				Frequencies: map[string]int64{
					"baz": 6,
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			c.initialDistribution.CountSample(c.sampleValue)
			if e, a := c.expectedDistribution, c.initialDistribution; !reflect.DeepEqual(e, a) {
				t.Errorf("expected %v, but received %v", e, a)
			}
		})
	}
}
