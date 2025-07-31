// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package nvme

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// For unit testing
var osReadFile = os.ReadFile
var osReadDir = os.ReadDir

func (u *Util) GetAllDevices() ([]DeviceFileAttributes, error) {
	entries, err := osReadDir(devDirectoryPath)
	if err != nil {
		return nil, err
	}

	devices := []DeviceFileAttributes{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), nvmeDevicePrefix) {
			device, err := ParseNvmeDeviceFileName(entry.Name())
			if err == nil {
				devices = append(devices, device)
			}
		}
	}

	return devices, nil
}

func (u *Util) GetDeviceSerial(device *DeviceFileAttributes) (string, error) {
	deviceName, err := device.BaseDeviceName()
	if err != nil {
		return "", err
	}
	data, err := osReadFile(fmt.Sprintf("%s/%s/serial", nvmeSysDirectoryPath, deviceName))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (u *Util) GetDeviceModel(device *DeviceFileAttributes) (string, error) {
	deviceName, err := device.BaseDeviceName()
	if err != nil {
		return "", err
	}
	data, err := osReadFile(fmt.Sprintf("%s/%s/model", nvmeSysDirectoryPath, deviceName))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (u *Util) IsEbsDevice(device *DeviceFileAttributes) (bool, error) {
	model, err := u.GetDeviceModel(device)
	if err != nil {
		return false, err
	}
	return model == ebsNvmeModelName, nil
}

func (u *Util) IsInstanceStoreDevice(device *DeviceFileAttributes) (bool, error) {
	// First check the model name
	model, err := u.GetDeviceModel(device)
	if err != nil {
		return false, fmt.Errorf("failed to get device model for %s: %w", device.DeviceName(), err)
	}
	if model != instanceStoreNvmeModelName {
		return false, nil
	}

	// If model name matches, validate the magic number from log page 0xC0
	devicePath, err := u.DevicePath(device.DeviceName())
	if err != nil {
		return false, fmt.Errorf("failed to get device path for %s: %w", device.DeviceName(), err)
	}

	// Try to read the log page and validate the magic number
	// This confirms the device is actually an Instance Store device
	_, err = GetInstanceStoreMetrics(devicePath)
	if err != nil {
		// Check if this is a magic number validation error specifically
		if errors.Is(err, ErrInvalidInstanceStoreMagic) {
			// Device has correct model name but wrong magic number - this is suspicious
			return false, fmt.Errorf("device %s has Instance Store model name but invalid magic number: %w", device.DeviceName(), err)
		}
		// For other errors (permissions, device access, etc.), we assume it's not an Instance Store device
		// but don't propagate the error as this is expected for non-Instance Store devices
		return false, nil
	}

	return true, nil
}

func (u *Util) DevicePath(device string) (string, error) {
	return filepath.Join(devDirectoryPath, device), nil
}
