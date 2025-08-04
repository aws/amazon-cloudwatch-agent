// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceType_String(t *testing.T) {
	tests := []struct {
		name     string
		dt       DeviceType
		expected string
	}{
		{
			name:     "EBS device type",
			dt:       DeviceTypeEBS,
			expected: "ebs",
		},
		{
			name:     "Instance Store device type",
			dt:       DeviceTypeInstanceStore,
			expected: "instance_store",
		},
		{
			name:     "Unknown device type",
			dt:       DeviceTypeUnknown,
			expected: "unknown",
		},
		{
			name:     "Invalid device type",
			dt:       DeviceType(999),
			expected: "invalid_device_type_999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dt.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeviceType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		dt       DeviceType
		expected bool
	}{
		{
			name:     "EBS is valid",
			dt:       DeviceTypeEBS,
			expected: true,
		},
		{
			name:     "Instance Store is valid",
			dt:       DeviceTypeInstanceStore,
			expected: true,
		},
		{
			name:     "Unknown is not valid",
			dt:       DeviceTypeUnknown,
			expected: false,
		},
		{
			name:     "Invalid value is not valid",
			dt:       DeviceType(999),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dt.IsValid()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseDeviceType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected DeviceType
	}{
		{
			name:     "Parse EBS",
			input:    "ebs",
			expected: DeviceTypeEBS,
		},
		{
			name:     "Parse Instance Store",
			input:    "instance_store",
			expected: DeviceTypeInstanceStore,
		},
		{
			name:     "Parse unknown string",
			input:    "unknown_device",
			expected: DeviceTypeUnknown,
		},
		{
			name:     "Parse empty string",
			input:    "",
			expected: DeviceTypeUnknown,
		},
		{
			name:     "Parse case sensitive",
			input:    "EBS",
			expected: DeviceTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDeviceType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeviceType_MarshalText(t *testing.T) {
	tests := []struct {
		name     string
		dt       DeviceType
		expected string
	}{
		{
			name:     "Marshal EBS",
			dt:       DeviceTypeEBS,
			expected: "ebs",
		},
		{
			name:     "Marshal Instance Store",
			dt:       DeviceTypeInstanceStore,
			expected: "instance_store",
		},
		{
			name:     "Marshal Unknown",
			dt:       DeviceTypeUnknown,
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.dt.MarshalText()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestDeviceType_UnmarshalText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected DeviceType
	}{
		{
			name:     "Unmarshal EBS",
			input:    "ebs",
			expected: DeviceTypeEBS,
		},
		{
			name:     "Unmarshal Instance Store",
			input:    "instance_store",
			expected: DeviceTypeInstanceStore,
		},
		{
			name:     "Unmarshal unknown",
			input:    "invalid",
			expected: DeviceTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dt DeviceType
			err := dt.UnmarshalText([]byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, dt)
		})
	}
}

func TestDeviceType_JSONSerialization(t *testing.T) {
	// Test JSON marshaling
	type testStruct struct {
		DeviceType DeviceType `json:"device_type"`
	}

	tests := []struct {
		name     string
		input    testStruct
		expected string
	}{
		{
			name:     "JSON marshal EBS",
			input:    testStruct{DeviceType: DeviceTypeEBS},
			expected: `{"device_type":"ebs"}`,
		},
		{
			name:     "JSON marshal Instance Store",
			input:    testStruct{DeviceType: DeviceTypeInstanceStore},
			expected: `{"device_type":"instance_store"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := json.Marshal(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))

			// Test round-trip
			var unmarshaled testStruct
			err = json.Unmarshal(result, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.input.DeviceType, unmarshaled.DeviceType)
		})
	}
}

func TestDeviceType_Constants(t *testing.T) {
	// Ensure constants have expected values
	assert.Equal(t, DeviceType(0), DeviceTypeUnknown)
	assert.Equal(t, DeviceType(1), DeviceTypeEBS)
	assert.Equal(t, DeviceType(2), DeviceTypeInstanceStore)

	// Ensure string representations are correct
	assert.Equal(t, "unknown", DeviceTypeUnknown.String())
	assert.Equal(t, "ebs", DeviceTypeEBS.String())
	assert.Equal(t, "instance_store", DeviceTypeInstanceStore.String())
}

func TestDeviceType_Roundtrip(t *testing.T) {
	// Test that parsing and string conversion are inverse operations
	deviceTypes := []DeviceType{
		DeviceTypeEBS,
		DeviceTypeInstanceStore,
		DeviceTypeUnknown,
	}

	for _, dt := range deviceTypes {
		t.Run(dt.String(), func(t *testing.T) {
			// Convert to string and back
			str := dt.String()
			parsed := ParseDeviceType(str)
			assert.Equal(t, dt, parsed, "Round-trip conversion should preserve value")
		})
	}
}
