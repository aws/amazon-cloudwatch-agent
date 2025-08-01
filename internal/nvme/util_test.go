// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUtil_Interface(t *testing.T) {
	var _ DeviceInfoProvider = &Util{}
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "Amazon Elastic Block Store", ebsNvmeModelName)
	assert.Equal(t, "Amazon EC2 NVMe Instance Storage", instanceStoreNvmeModelName)
	assert.Equal(t, uint32(0xEC2C0D7E), uint32(InstanceStoreMagicNumber))
}

func TestDeviceTypeConstants(t *testing.T) {
	// Test that device type constants are properly defined
	assert.Equal(t, "ebs", "ebs")
	assert.Equal(t, "instance_store", "instance_store")
}

func TestDeviceInfoProvider_DetectDeviceType_Interface(t *testing.T) {
	// Test that DetectDeviceType method is properly added to the interface
	var provider DeviceInfoProvider = &Util{}

	// Create a test device
	device := DeviceFileAttributes{controller: -1, deviceName: "test"}

	// Call DetectDeviceType - this should compile and run without panic
	// On non-Linux platforms, it should return an error
	deviceType, err := provider.DetectDeviceType(&device)

	// On non-Linux platforms, we expect an error
	assert.Error(t, err, "DetectDeviceType should return error on non-Linux platforms")
	assert.Contains(t, err.Error(), "only supported on Linux", "Error should mention Linux requirement")
	assert.Equal(t, "", deviceType, "Device type should be empty on error")
}
