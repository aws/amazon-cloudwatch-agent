// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterReservedKeys_ReservedKeyFiltering(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]any
	}{
		{
			name: "Reserved keys are filtered out",
			input: map[string]interface{}{
				"InstanceType":            "t3.medium",
				"aws:StorageResolution":   "true",
				"aws:AggregationInterval": "60",
				"VolumeId":                "vol-123",
				"HardcodedName":           "HardcodedValue",
			},
			expected: map[string]any{
				"InstanceType":  "t3.medium",
				"HardcodedName": "HardcodedValue",
			},
		},
		{
			name: "No reserved keys present",
			input: map[string]interface{}{
				"InstanceType":  "t3.medium",
				"HardcodedName": "HardcodedValue",
				"Environment":   "production",
			},
			expected: map[string]any{
				"InstanceType":  "t3.medium",
				"HardcodedName": "HardcodedValue",
				"Environment":   "production",
			},
		},
		{
			name: "All reserved keys filtered",
			input: map[string]interface{}{
				"aws:StorageResolution":   "true",
				"aws:AggregationInterval": "60",
				"VolumeId":                "vol-123",
			},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterReservedKeys(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
