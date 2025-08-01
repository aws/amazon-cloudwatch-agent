// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNvmeDeviceFileName(t *testing.T) {
	tests := []struct {
		name          string
		device        string
		expectedError bool
		expectedCtrl  int
		expectedNS    int
		expectedPart  int
	}{
		{
			name:         "valid device with namespace and partition",
			device:       "nvme0n1p1",
			expectedCtrl: 0,
			expectedNS:   1,
			expectedPart: 1,
		},
		{
			name:         "valid device with namespace only",
			device:       "nvme1n2",
			expectedCtrl: 1,
			expectedNS:   2,
			expectedPart: -1,
		},
		{
			name:          "invalid device name",
			device:        "sda1",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device, err := ParseNvmeDeviceFileName(tt.device)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCtrl, device.Controller())
			assert.Equal(t, tt.expectedNS, device.Namespace())
			assert.Equal(t, tt.expectedPart, device.Partition())
			assert.Equal(t, tt.device, device.DeviceName())
		})
	}
}

func TestDeviceFileAttributes_BaseDeviceName(t *testing.T) {
	device, err := ParseNvmeDeviceFileName("nvme0n1p1")
	assert.NoError(t, err)

	baseName, err := device.BaseDeviceName()
	assert.NoError(t, err)
	assert.Equal(t, "nvme0", baseName)
}
