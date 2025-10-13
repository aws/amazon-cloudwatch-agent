// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEKSClusterName(t *testing.T) {
	tests := []struct {
		name           string
		sectionKey     string
		input          map[string]interface{}
		expectedResult string
	}{
		{
			name:       "Cluster name from config",
			sectionKey: "cluster_name",
			input: map[string]interface{}{
				"cluster_name": "my-test-cluster",
			},
			expectedResult: "my-test-cluster",
		},
		{
			name:           "Empty config falls back to EC2 tags",
			sectionKey:     "cluster_name",
			input:          map[string]interface{}{},
			expectedResult: "", // Will be empty since we don't have real EC2 metadata in test
		},
		{
			name:       "Missing key falls back to EC2 tags",
			sectionKey: "cluster_name",
			input: map[string]interface{}{
				"other_key": "other_value",
			},
			expectedResult: "", // Will be empty since we don't have real EC2 metadata in test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetEKSClusterName(tt.sectionKey, tt.input)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetClusterNameFromEc2Tagger(t *testing.T) {
	// This will return empty string in test environment since there's no real EC2 metadata
	result := GetClusterNameFromEc2Tagger()
	assert.Equal(t, "", result)
}
