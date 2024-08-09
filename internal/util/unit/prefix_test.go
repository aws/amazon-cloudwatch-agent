// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package unit

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricPrefix(t *testing.T) {
	testCases := []struct {
		prefix string
		value  float64
	}{
		{"Ki", -1},
		{"k", 1e3},
		{"M", 1e6},
		{"G", 1e9},
		{"T", 1e12},
	}
	for _, testCase := range testCases {
		got := MetricPrefix(testCase.prefix)
		assert.Equal(t, testCase.value, got.Scale())
		assert.Equal(t, testCase.prefix, got.String())
	}
	assert.Len(t, MetricPrefixes, 4)
}

func TestBinaryPrefix(t *testing.T) {
	testCases := []struct {
		prefix string
		value  float64
	}{
		{"k", -1},
		{"Ki", math.Pow(2, 10)},
		{"Mi", math.Pow(2, 20)},
		{"Gi", math.Pow(2, 30)},
		{"Ti", math.Pow(2, 40)},
	}
	for _, testCase := range testCases {
		got := BinaryPrefix(testCase.prefix)
		assert.Equal(t, testCase.value, got.Scale())
		assert.Equal(t, testCase.prefix, got.String())
	}
	assert.Len(t, BinaryPrefixes, 4)
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
	}{
		{BinaryPrefixKibi, MetricPrefixKilo, 1.024},
		{BinaryPrefixGibi, MetricPrefixGiga, 1.073741824},
	}
	for _, testCase := range testCases {
		got, scale, err = ConvertToMetric(testCase.prefix)
		assert.NoError(t, err)
		assert.Equal(t, testCase.metricPrefix, got)
		assert.Equal(t, testCase.scale, scale)
	}
}
