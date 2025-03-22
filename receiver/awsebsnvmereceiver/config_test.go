// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsebsnvmereceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	c := Config{}
	err := c.Validate()
	require.NotNil(t, err)
}

func TestConfigWithResources(t *testing.T) {
	testCases := []struct {
		name      string
		resources []string
	}{
		{
			name:      "empty resources",
			resources: []string{},
		},
		{
			name:      "single resource",
			resources: []string{"nvme0n1"},
		},
		{
			name:      "multiple resources",
			resources: []string{"nvme0n1", "nvme1n1"},
		},
		{
			name:      "wildcard",
			resources: []string{"*"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Resources = tc.resources

			// Just verify we can set the resources field
			assert.Equal(t, tc.resources, cfg.Resources)
		})
	}
}
