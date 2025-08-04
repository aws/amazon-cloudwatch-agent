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
		// Enhance error with context for better debugging
		if os.IsPermission(err) {
			return nil, WrapError(ErrDeviceAccessDenied, "device discovery", devDirectoryPath, map[string]string{
				"suggestion": "ensure read permissions on /dev directory",
			})
		}
		if os.IsNotExist(err) {
			return nil, WrapError(ErrDeviceNotFound, "device discovery", devDirectoryPath, map[string]string{
				"suggestion": "/dev directory not found - check if running in container",
			})
		}
		return nil, WrapError(ErrDeviceAccess, "device discovery", devDirectoryPath, nil)
	}

	devices := []DeviceFileAttributes{}
	parseErrors := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), nvmeDevicePrefix) {
			device, err := ParseNvmeDeviceFileName(entry.Name())
			if err == nil {
				devices = append(devices, device)
			} else {
				parseErrors++
				// Log parse errors but continue with other devices
				// The caller should handle logging since we don't have a logger at this level
			}
		}
	}

	// If we found some devices but had parse errors, still return the valid devices
	// The caller can decide how to handle partial success
	return devices, nil
}

func (u *Util) GetDeviceSerial(device *DeviceFileAttributes) (string, error) {
	deviceName, err := device.BaseDeviceName()
	if err != nil {
		return "", WrapError(err, "get device serial", device.DeviceName(), nil)
	}

	serialPath := fmt.Sprintf("%s/%s/serial", nvmeSysDirectoryPath, deviceName)
	data, err := osReadFile(serialPath)
	if err != nil {
		// Enhance error with context for better debugging
		if os.IsPermission(err) {
			return "", WrapError(ErrDeviceAccessDenied, "read device serial", serialPath, map[string]string{
				"suggestion": "ensure read permissions on sysfs",
			})
		}
		if os.IsNotExist(err) {
			return "", WrapError(ErrDeviceNotFound, "read device serial", serialPath, map[string]string{
				"suggestion": "device may have been removed or sysfs not mounted",
			})
		}
		return "", WrapError(ErrDeviceAccess, "read device serial", serialPath, nil)
	}

	serial := strings.TrimSpace(string(data))
	if serial == "" {
		return "", WrapError(ErrCorruptedData, "read device serial", serialPath, map[string]string{
			"issue": "empty serial number",
		})
	}

	return serial, nil
}

func (u *Util) GetDeviceModel(device *DeviceFileAttributes) (string, error) {
	deviceName, err := device.BaseDeviceName()
	if err != nil {
		return "", WrapError(err, "get device model", device.DeviceName(), nil)
	}

	modelPath := fmt.Sprintf("%s/%s/model", nvmeSysDirectoryPath, deviceName)
	data, err := osReadFile(modelPath)
	if err != nil {
		// Enhance error with context for better debugging
		if os.IsPermission(err) {
			return "", WrapError(ErrDeviceAccessDenied, "read device model", modelPath, map[string]string{
				"suggestion": "ensure read permissions on sysfs",
			})
		}
		if os.IsNotExist(err) {
			return "", WrapError(ErrDeviceNotFound, "read device model", modelPath, map[string]string{
				"suggestion": "device may have been removed or sysfs not mounted",
			})
		}
		return "", WrapError(ErrDeviceAccess, "read device model", modelPath, nil)
	}

	model := strings.TrimSpace(string(data))
	if model == "" {
		return "", WrapError(ErrCorruptedData, "read device model", modelPath, map[string]string{
			"issue": "empty model name",
		})
	}

	return model, nil
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
		return false, WrapError(err, "Instance Store device detection", device.DeviceName(), map[string]string{
			"step": "model name retrieval",
		})
	}
	if model != instanceStoreNvmeModelName {
		return false, nil
	}

	// If model name matches, validate the magic number from log page 0xC0
	devicePath, err := u.DevicePath(device.DeviceName())
	if err != nil {
		return false, WrapError(err, "Instance Store device detection", device.DeviceName(), map[string]string{
			"step": "device path resolution",
		})
	}

	// Try to read the log page and validate the magic number
	// This confirms the device is actually an Instance Store device
	_, err = GetInstanceStoreMetrics(devicePath)
	if err != nil {
		// Check if this is a magic number validation error specifically
		if errors.Is(err, ErrInvalidInstanceStoreMagic) {
			// Device has correct model name but wrong magic number - this is suspicious
			return false, WrapError(err, "Instance Store device detection", device.DeviceName(), map[string]string{
				"step":  "magic number validation",
				"model": model,
				"issue": "model name matches but magic number is invalid",
			})
		}

		// For recoverable errors (permissions, device busy, etc.), we can't determine device type
		if IsRecoverableError(err) {
			return false, WrapError(ErrTemporaryFailure, "Instance Store device detection", device.DeviceName(), map[string]string{
				"step":        "magic number validation",
				"model":       model,
				"underlying":  err.Error(),
				"recoverable": "true",
			})
		}

		// For other errors (device access, etc.), we assume it's not an Instance Store device
		// but don't propagate the error as this is expected for non-Instance Store devices
		return false, nil
	}

	return true, nil
}

func (u *Util) DetectDeviceType(device *DeviceFileAttributes) (string, error) {
	deviceName := device.DeviceName()

	// First check if it's an EBS device
	isEbs, err := u.IsEbsDevice(device)
	if err != nil {
		return "", WrapError(err, "device type detection", deviceName, map[string]string{
			"step": "EBS device check",
		})
	}
	if isEbs {
		return "ebs", nil
	}

	// Then check if it's an Instance Store device
	isInstanceStore, err := u.IsInstanceStoreDevice(device)
	if err != nil {
		// Check if this is a recoverable error that we should propagate
		if IsRecoverableError(err) {
			return "", WrapError(err, "device type detection", deviceName, map[string]string{
				"step":        "Instance Store device check",
				"recoverable": "true",
			})
		}

		// For non-recoverable errors in Instance Store detection, log but continue
		// This allows the function to still return "unknown" rather than failing completely
	}
	if isInstanceStore {
		return "instance_store", nil
	}

	// If neither EBS nor Instance Store, try to get model for better error context
	model, modelErr := u.GetDeviceModel(device)
	if modelErr != nil {
		// If we can't even get the model, return a generic unknown device error
		return "", WrapError(ErrInvalidDeviceState, "device type detection", deviceName, map[string]string{
			"step":  "model retrieval for unknown device",
			"issue": "unable to determine device type or model",
		})
	}

	return "", WrapError(ErrInvalidDeviceState, "device type detection", deviceName, map[string]string{
		"step":  "device type classification",
		"model": model,
		"issue": "device model does not match known EBS or Instance Store patterns",
	})
}

func (u *Util) DevicePath(device string) (string, error) {
	// Sanitize input
	originalDevice := device
	device = strings.TrimSpace(device)
	if device == "" {
		return "", fmt.Errorf("device name cannot be empty")
	}

	// Security check: Detect null byte injection and control characters
	if strings.Contains(device, "\x00") {
		return "", fmt.Errorf("device name cannot contain null bytes")
	}

	for i, char := range device {
		if char < 32 && char != 9 { // Allow tab but not other control characters
			return "", fmt.Errorf("device name contains invalid control character at position %d (code %d)", i, char)
		}
	}

	// Security check: Validate device name doesn't contain path traversal attempts
	if strings.Contains(device, "..") || strings.Contains(device, "/") {
		return "", fmt.Errorf("device name cannot contain path separators or traversal sequences")
	}

	// Security check: Detect other suspicious patterns
	if strings.Contains(device, "\\") {
		return "", fmt.Errorf("device name cannot contain backslashes")
	}

	// Security check: Validate device name contains only valid characters
	for i, char := range device {
		if !isValidDeviceNameChar(char) {
			return "", fmt.Errorf("device name contains invalid character '%c' at position %d", char, i)
		}
	}

	// Security check: Validate device name length to prevent buffer overflow
	if len(device) > 32 {
		return "", fmt.Errorf("device name exceeds maximum length of 32 characters (got %d)", len(device))
	}

	// Security check: Validate device name follows expected NVMe pattern
	if err := ValidateDeviceNamePattern(device); err != nil {
		return "", fmt.Errorf("invalid device name pattern: %w", err)
	}

	// Construct and validate the full path
	fullPath := filepath.Join(devDirectoryPath, device)

	// Security check: Ensure the path is still within /dev after joining
	cleanPath := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleanPath, devDirectoryPath+"/") && cleanPath != devDirectoryPath {
		return "", fmt.Errorf("device path escapes /dev directory (resolved to: %s)", cleanPath)
	}

	// Security check: Ensure the resolved path matches expected format
	if cleanPath != fullPath {
		return "", fmt.Errorf("device path resolution mismatch (expected: %s, resolved: %s)", fullPath, cleanPath)
	}

	// Log if input was sanitized (would be handled by caller with logger)
	_ = originalDevice // Prevent unused variable warning

	return cleanPath, nil
}

// isValidDeviceNameChar checks if a character is valid in a device name
func isValidDeviceNameChar(char rune) bool {
	// Allow alphanumeric characters and common device name characters
	// For NVMe devices, we're more restrictive: only digits, lowercase letters, 'n', and 'p'
	return (char >= '0' && char <= '9') ||
		(char >= 'a' && char <= 'z') ||
		char == 'n' || char == 'p'
}

// ValidateDeviceNamePattern validates that the device name follows expected NVMe naming patterns
// This function is exported for testing purposes
func ValidateDeviceNamePattern(deviceName string) error {
	// NVMe device names should follow pattern: nvme<controller>n<namespace>[p<partition>]
	// But here we only get the part after "nvme", so: <controller>n<namespace>[p<partition>]
	// Examples: 0n1, 1n2p1, 10n1p5

	if len(deviceName) < 3 {
		return fmt.Errorf("device name too short for valid NVMe pattern (minimum 3 characters)")
	}

	// Find the 'n' separator
	nIndex := strings.Index(deviceName, "n")
	if nIndex == -1 {
		return fmt.Errorf("device name must contain 'n' separator (controller<n>namespace)")
	}

	// Validate controller part (before 'n')
	controllerPart := deviceName[:nIndex]
	if len(controllerPart) == 0 {
		return fmt.Errorf("missing controller number")
	}
	for i, char := range controllerPart {
		if char < '0' || char > '9' {
			return fmt.Errorf("controller part contains non-digit character '%c' at position %d", char, i)
		}
	}

	// Validate namespace and optional partition part (after 'n')
	remainingPart := deviceName[nIndex+1:]
	if len(remainingPart) == 0 {
		return fmt.Errorf("missing namespace number")
	}

	// Check for partition separator 'p'
	pIndex := strings.Index(remainingPart, "p")
	if pIndex == -1 {
		// No partition, validate entire remaining part as namespace
		for i, char := range remainingPart {
			if char < '0' || char > '9' {
				return fmt.Errorf("namespace part contains non-digit character '%c' at position %d", char, i)
			}
		}
	} else {
		// Has partition, validate namespace part (before 'p')
		namespacePart := remainingPart[:pIndex]
		if len(namespacePart) == 0 {
			return fmt.Errorf("missing namespace number before partition")
		}
		for i, char := range namespacePart {
			if char < '0' || char > '9' {
				return fmt.Errorf("namespace part contains non-digit character '%c' at position %d", char, i)
			}
		}

		// Validate partition part (after 'p')
		partitionPart := remainingPart[pIndex+1:]
		if len(partitionPart) == 0 {
			return fmt.Errorf("missing partition number after 'p'")
		}
		for i, char := range partitionPart {
			if char < '0' || char > '9' {
				return fmt.Errorf("partition part contains non-digit character '%c' at position %d", char, i)
			}
		}

		// Security check: Ensure no additional 'p' separators
		if strings.Count(remainingPart, "p") > 1 {
			return fmt.Errorf("device name contains multiple partition separators")
		}
	}

	// Security check: Ensure no additional 'n' separators
	if strings.Count(deviceName, "n") > 1 {
		return fmt.Errorf("device name contains multiple namespace separators")
	}

	return nil
}
