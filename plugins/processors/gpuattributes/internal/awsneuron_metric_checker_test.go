// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"testing"
)

func TestAwsNeuronMetricModifier_IsProcessedNeuronMetric(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "container_neuroncore_prefix",
			input:    "container_neuroncore_metric",
			expected: true,
		},
		{
			name:     "pod_neuroncore_prefix",
			input:    "pod_neuroncore_metric",
			expected: true,
		},
		{
			name:     "node_neuroncore_prefix",
			input:    "node_neuroncore_metric",
			expected: true,
		},
		{
			name:     "container_neurondevice_prefix",
			input:    "container_neurondevice_metric",
			expected: true,
		},
		{
			name:     "pod_neurondevice_prefix",
			input:    "pod_neurondevice_metric",
			expected: true,
		},
		{
			name:     "node_neurondevice_prefix",
			input:    "node_neurondevice_metric",
			expected: true,
		},
		{
			name:     "node_neuron_prefix",
			input:    "node_neuron_metric",
			expected: true,
		},
		{
			name:     "container_neuron_prefix",
			input:    "container_neuron_metric",
			expected: false,
		},
		{
			name:     "other_prefix",
			input:    "other_metric",
			expected: false,
		},
	}

	md := NewAwsNeuronMetricChecker()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := md.IsProcessedNeuronMetric(test.input)
			if result != test.expected {
				t.Errorf("IsProcessedNeuronMetric(%q) = %v, expected %v", test.input, result, test.expected)
			}
		})
	}
}
