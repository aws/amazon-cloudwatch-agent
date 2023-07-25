// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package unit

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricPrefix(t *testing.T) {
	testCases := []struct {
		prefix string
		value  float64
	}{
		{"Ki", -1},
		{"k", 1e3},
		{"G", 1e9},
	}
	for _, testCase := range testCases {
		got := MetricPrefix(testCase.prefix)
		assert.Equal(t, testCase.value, got.Value())
	}
}

func TestBinaryPrefix(t *testing.T) {
	testCases := []struct {
		prefix string
		value  float64
	}{
		{"k", -1},
		{"Ki", 1024},
		{"Gi", 1073741824},
	}
	for _, testCase := range testCases {
		got := BinaryPrefix(testCase.prefix)
		assert.Equal(t, testCase.value, got.Value())
	}
}

func TestConvertBinaryToMetric(t *testing.T) {
	got, scale, err := ConvertToMetric("k")
	assert.Error(t, err)
	assert.EqualValues(t, "", got)
	assert.EqualValues(t, -1, scale)
	testCases := []struct {
		prefix       BinaryPrefix
		metricPrefix MetricPrefix
		scale        float64
		epsilon      float64
	}{
		{BinaryPrefixKibi, MetricPrefixKilo, 1.024, 0},
		{BinaryPrefixGibi, MetricPrefixGiga, 1.073, 0.001},
	}
	for _, testCase := range testCases {
		got, scale, err = ConvertToMetric(testCase.prefix)
		require.NoError(t, err)
		assert.Equal(t, testCase.metricPrefix, got)
		assert.GreaterOrEqual(t, testCase.epsilon, math.Abs(testCase.scale-scale))
	}
}
