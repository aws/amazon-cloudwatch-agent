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

func (u *Util) DetectDeviceType(device *DeviceFileAttributes) (string, error) {
	// First check if it's an EBS device
	isEbs, err := u.IsEbsDevice(device)
	if err != nil {
		return "", fmt.Errorf("failed to check if device %s is EBS: %w", device.DeviceName(), err)
	}
	if isEbs {
		return "ebs", nil
	}

	// Then check if it's an Instance Store device
	isInstanceStore, err := u.IsInstanceStoreDevice(device)
	if err != nil {
		return "", fmt.Errorf("failed to check if device %s is Instance Store: %w", device.DeviceName(), err)
	}
	if isInstanceStore {
		return "instance_store", nil
	}

	// If neither EBS nor Instance Store, return unknown
	model, err := u.GetDeviceModel(device)
	if err != nil {
		return "", fmt.Errorf("failed to get device model for %s: %w", device.DeviceName(), err)
	}
	return "", fmt.Errorf("unknown device type for %s with model '%s'", device.DeviceName(), model)
}

func (u *Util) DevicePath(device string) (string, error) {
	// Sanitize input
	device = strings.TrimSpace(device)
	if device == "" {
		return "", fmt.Errorf("device name cannot be empty")
	}

	// Validate device name doesn't contain path traversal attempts
	if strings.Contains(device, "..") || strings.Contains(device, "/") {
		return "", fmt.Errorf("device name cannot contain path separators or traversal sequences")
	}

	// Validate device name contains only valid characters
	for _, char := range device {
		if !isValidDeviceNameChar(char) {
			return "", fmt.Errorf("device name contains invalid character: %c", char)
		}
	}

	// Validate device name length
	if len(device) > 32 {
		return "", fmt.Errorf("device name exceeds maximum length of 32 characters")
	}

	// Construct and validate the full path
	fullPath := filepath.Join(devDirectoryPath, device)

	// Ensure the path is still within /dev after joining
	cleanPath := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleanPath, devDirectoryPath+"/") && cleanPath != devDirectoryPath {
		return "", fmt.Errorf("device path escapes /dev directory")
	}

	return cleanPath, nil
}

// isValidDeviceNameChar checks if a character is valid in a device name
func isValidDeviceNameChar(char rune) bool {
	// Allow alphanumeric characters and common device name characters
	return (char >= '0' && char <= '9') ||
		(char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		char == '_' || char == '-'
}
