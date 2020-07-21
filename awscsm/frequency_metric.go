// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsmmetrics

// FrequencyMetric represents a metric that is a numeric value.
type FrequencyMetric struct {
	Name        string
	Frequencies map[string]int64
}

// NewFrequencyMetric will return a new metric with instantiated maps
func NewFrequencyMetric(name string) FrequencyMetric {
	return FrequencyMetric{
		Name:        name,
		Frequencies: map[string]int64{},
	}
}

// CountSample will add all frequencies in the metric.
// If rhs is nil, we will return immediately with no error. We will treat
// nil as zero count.
func (m *FrequencyMetric) CountSample(k string) {
	if _, ok := m.Frequencies[k]; !ok {
		m.Frequencies[k] = 0
	}

	m.Frequencies[k]++
}

// FrequencyMetrics is a collection of metrics
type FrequencyMetrics map[string]FrequencyMetric
