// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"testing"
)

func TestIsTestDocument(t *testing.T) {
	testCases := []struct {
		name     string
		docName  string
		expected bool
	}{
		{
			name:     "Test document with correct prefix",
			docName:  "Test-AmazonCloudWatch-ManageAgent-abc123",
			expected: true,
		},
		{
			name:     "Production AmazonCloudWatch-ManageAgent document",
			docName:  "AmazonCloudWatch-ManageAgent",
			expected: false,
		},
		{
			name:     "Regular document",
			docName:  "MyCustomDocument",
			expected: false,
		},
		{
			name:     "Empty document name",
			docName:  "",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isTestDocument(tc.docName)
			if result != tc.expected {
				t.Errorf("isTestDocument(%q) = %v, expected %v", tc.docName, result, tc.expected)
			}
		})
	}
}

func TestIsTestParameter(t *testing.T) {
	testCases := []struct {
		name      string
		paramName string
		expected  bool
	}{
		{
			name:      "Test parameter agentConfig1",
			paramName: "agentConfig1",
			expected:  true,
		},
		{
			name:      "Test parameter agentConfig2",
			paramName: "agentConfig2",
			expected:  true,
		},
		{
			name:      "Test parameter MetricRenameSSM",
			paramName: "MetricRenameSSM",
			expected:  true,
		},
		{
			name:      "Regular parameter",
			paramName: "regular-parameter",
			expected:  false,
		},
		{
			name:      "Parameter with agentConfig in name but not exact match",
			paramName: "myagentConfig1setting",
			expected:  false,
		},
		{
			name:      "Empty parameter name",
			paramName: "",
			expected:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isTestParameter(tc.paramName)
			if result != tc.expected {
				t.Errorf("isTestParameter(%q) = %v, expected %v", tc.paramName, result, tc.expected)
			}
		})
	}
}
