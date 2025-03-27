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

func TestConfigWithDevices(t *testing.T) {
	testCases := []struct {
		name    string
		devices []string
	}{
		{
			name:    "empty devices",
			devices: []string{},
		},
		{
			name:    "single device",
			devices: []string{"nvme0n1"},
		},
		{
			name:    "multiple devices",
			devices: []string{"nvme0n1", "nvme1n1"},
		},
		{
			name:    "wildcard",
			devices: []string{"*"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Devices = tc.devices

			// Just verify we can set the devices field
			assert.Equal(t, tc.devices, cfg.Devices)
		})
	}
}
