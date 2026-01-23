// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudmetadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCloudProviderString(t *testing.T) {
	tests := []struct {
		name     string
		provider CloudProvider
		expected string
	}{
		{"AWS", CloudProviderAWS, "AWS"},
		{"Azure", CloudProviderAzure, "Azure"},
		{"Unknown", CloudProviderUnknown, "Unknown"},
		{"Invalid", CloudProvider(100), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.provider.String())
		})
	}
}
