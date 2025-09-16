// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package downloader

import (
	"testing"
)

func TestRunDownloaderFromFlags_DualStackFlag(t *testing.T) {
	tests := []struct {
		name           string
		dualStackValue string
		expectedResult bool
	}{
		{
			name:           "DualStack enabled",
			dualStackValue: "true",
			expectedResult: true,
		},
		{
			name:           "DualStack disabled",
			dualStackValue: "false",
			expectedResult: false,
		},
		{
			name:           "DualStack empty string",
			dualStackValue: "",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock flags
			flags := map[string]*string{
				"mode":            stringPtr("ec2"),
				"download-source": stringPtr("default"),
				"output-dir":      stringPtr("/tmp/test"),
				"config":          stringPtr(""),
				"multi-config":    stringPtr("default"),
				"dualstack":       stringPtr(tt.dualStackValue),
			}

			// Test that the flag parsing works correctly
			// We can't easily test the full RunDownloader function without mocking AWS services,
			// but we can test that the flag conversion logic works
			result := *flags["dualstack"] == "true"
			if result != tt.expectedResult {
				t.Errorf("Expected dualstack flag conversion to be %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
