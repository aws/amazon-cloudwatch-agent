// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package debugger

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckEndpoints(t *testing.T) {
	tests := []struct {
		name                    string
		config                  map[string]interface{}
		ssm                     bool
		expectedMetricsEndpoint string
		expectedLogsEndpoint    string
		expectedOutputContains  []string
	}{
		{
			name:                    "Nil config",
			config:                  nil,
			ssm:                     true,
			expectedMetricsEndpoint: "No configuration available",
			expectedLogsEndpoint:    "No configuration available",
			expectedOutputContains:  []string{"No configuration available"},
		},
		{
			name:                    "Empty config",
			config:                  map[string]interface{}{},
			ssm:                     true,
			expectedMetricsEndpoint: "No metrics configuration found",
			expectedLogsEndpoint:    "No logs configuration found",
			expectedOutputContains:  []string{"=== Endpoint Configuration ===", "Metrics: No metrics configuration found", "Logs:    No logs configuration found"},
		},
		{
			name: "Config with custom endpoints",
			config: map[string]interface{}{
				"metrics": map[string]interface{}{
					"endpoint_override": "https://custom-metrics.amazonaws.com",
				},
				"logs": map[string]interface{}{
					"endpoint_override": "https://custom-logs.amazonaws.com",
				},
			},
			ssm:                     true,
			expectedMetricsEndpoint: "https://custom-metrics.amazonaws.com",
			expectedLogsEndpoint:    "https://custom-logs.amazonaws.com",
			expectedOutputContains:  []string{"Metrics: https://custom-metrics.amazonaws.com", "Logs:    https://custom-logs.amazonaws.com"},
		},
		{
			name: "Config with default endpoints",
			config: map[string]interface{}{
				"metrics": map[string]interface{}{},
				"logs":    map[string]interface{}{},
			},
			ssm:                     true,
			expectedMetricsEndpoint: "Default CloudWatch endpoint (no override)",
			expectedLogsEndpoint:    "Default CloudWatch Logs endpoint (no override)",
			expectedOutputContains:  []string{"Metrics: Default CloudWatch endpoint (no override)", "Logs:    Default CloudWatch Logs endpoint (no override)"},
		},
		{
			name: "Table format output",
			config: map[string]interface{}{
				"metrics": map[string]interface{}{
					"endpoint_override": "https://metrics.amazonaws.com",
				},
				"logs": map[string]interface{}{
					"endpoint_override": "https://logs.amazonaws.com",
				},
			},
			ssm:                     false,
			expectedMetricsEndpoint: "https://metrics.amazonaws.com",
			expectedLogsEndpoint:    "https://logs.amazonaws.com",
			expectedOutputContains:  []string{"┌", "│", "└", "Metrics", "Logs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			result, err := CheckEndpoints(&buf, tt.config, tt.ssm)

			assert.NoError(t, err, "CheckEndpoints should not return an error")
			assert.NotNil(t, result, "CheckEndpoints should return a result")

			assert.Equal(t, tt.expectedMetricsEndpoint, result.MetricsEndpoint, "Metrics endpoint should match expected")
			assert.Equal(t, tt.expectedLogsEndpoint, result.LogsEndpoint, "Logs endpoint should match expected")

			output := buf.String()
			for _, expectedContent := range tt.expectedOutputContains {
				assert.Contains(t, output, expectedContent, "Output should contain expected content")
			}
		})
	}
}

func TestCheckEndpointsSSMFormat(t *testing.T) {
	config := map[string]interface{}{
		"metrics": map[string]interface{}{
			"endpoint_override": "https://custom-metrics.amazonaws.com",
		},
		"logs": map[string]interface{}{
			"endpoint_override": "https://custom-logs.amazonaws.com",
		},
	}

	var buf bytes.Buffer
	result, err := CheckEndpoints(&buf, config, true)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	output := buf.String()

	assert.Contains(t, output, "=== Endpoint Configuration ===")
	assert.Contains(t, output, "Metrics: https://custom-metrics.amazonaws.com")
	assert.Contains(t, output, "Logs:    https://custom-logs.amazonaws.com")

	assert.NotContains(t, output, "┌")
	assert.NotContains(t, output, "│")
	assert.NotContains(t, output, "└")
}

func TestCheckEndpointsTableFormat(t *testing.T) {
	config := map[string]interface{}{
		"metrics": map[string]interface{}{
			"endpoint_override": "https://custom-metrics.amazonaws.com",
		},
		"logs": map[string]interface{}{
			"endpoint_override": "https://custom-logs.amazonaws.com",
		},
	}

	var buf bytes.Buffer
	result, err := CheckEndpoints(&buf, config, false)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	output := buf.String()

	// Table format should contain table characters
	assert.Contains(t, output, "┌")
	assert.Contains(t, output, "│")
	assert.Contains(t, output, "└")
	assert.Contains(t, output, "Metrics")
	assert.Contains(t, output, "Logs")
}
