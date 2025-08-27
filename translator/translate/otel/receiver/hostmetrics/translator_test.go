// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestTranslator_ID(t *testing.T) {
	translator := NewTranslator()
	expected := component.NewID(component.MustNewType("hostmetrics"))
	assert.Equal(t, expected, translator.ID())
}

func TestIsHostmetricsMetric(t *testing.T) {
	testCases := []struct {
		name     string
		metric   string
		expected bool
	}{
		{
			name:     "LoadAverageMetric",
			metric:   "load_average",
			expected: true,
		},
		{
			name:     "CPULoadAverageMetric",
			metric:   "cpu_load_average",
			expected: true,
		},
		{
			name:     "UsageIdleMetric",
			metric:   "usage_idle",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsHostmetricsMetric(tc.metric)
			assert.Equal(t, tc.expected, result)
		})
	}
}
