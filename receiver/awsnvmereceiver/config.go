// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
)

// Config defines the configuration for the unified AWS NVMe receiver
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Devices                        []string `mapstructure:"devices,omitempty"`
}

var _ component.Config = (*Config)(nil)

// Validate validates the receiver configuration
func (cfg *Config) Validate() error {
	if err := cfg.ControllerConfig.Validate(); err != nil {
		return err
	}

	return cfg.validateDevices()
}

// validateDevices validates device paths and wildcard support
func (cfg *Config) validateDevices() error {
	if len(cfg.Devices) == 0 {
		// Empty devices list is valid - defaults to auto-discovery
		return nil
	}

	for _, device := range cfg.Devices {
		if err := cfg.validateDevice(device); err != nil {
			return err
		}
	}

	return nil
}

// validateDevice validates a single device path with comprehensive security checks
func (cfg *Config) validateDevice(device string) error {
	if device == "" {
		return errors.New("device path cannot be empty")
	}

	// Support wildcard for auto-discovery
	if device == "*" {
		return nil
	}

	// Sanitize input by trimming whitespace and checking for null bytes
	originalDevice := device
	device = strings.TrimSpace(device)

	// Security check: Detect null byte injection attempts
	if strings.Contains(device, "\x00") {
		return errors.New("device path cannot contain null bytes")
	}

	// Security check: Detect control characters that could be used in attacks
	for _, char := range device {
		if char < 32 { // Reject all control characters including tab, LF, CR
			return fmt.Errorf("device path contains invalid control character (code %d)", char)
		}
	}

	// Validate device path format first
	if !strings.HasPrefix(device, "/dev/") {
		return errors.New("device path must start with /dev/")
	}

	// Security check: Comprehensive path traversal prevention
	if strings.Contains(device, "..") {
		return errors.New("device path cannot contain '..'")
	}
	if strings.Contains(device, "./") {
		return errors.New("device path cannot contain relative path components")
	}
	if strings.Contains(device, "//") {
		return errors.New("device path cannot contain double slashes")
	}
	if strings.Contains(device, "\\") {
		return errors.New("device path cannot contain backslashes")
	}

	// Security check: Prevent directory traversal attacks using filepath.Clean
	cleanPath := filepath.Clean(device)
	if cleanPath != device {
		return fmt.Errorf("device path contains invalid characters or sequences (cleaned: %s)", cleanPath)
	}

	// Security check: Validate absolute path doesn't escape /dev
	absPath, err := filepath.Abs(device)
	if err != nil {
		return fmt.Errorf("device path is not a valid absolute path: %w", err)
	}
	if !strings.HasPrefix(absPath, "/dev/") {
		return fmt.Errorf("device path must resolve to /dev/ directory (resolved to: %s)", absPath)
	}

	// Security check: Ensure the resolved path is still the same as input
	if absPath != device {
		return fmt.Errorf("device path resolution mismatch (input: %s, resolved: %s)", device, absPath)
	}

	// Validate NVMe device naming pattern with stricter validation
	if !strings.HasPrefix(device, "/dev/nvme") {
		return errors.New("device path must be an NVMe device (/dev/nvme*)")
	}

	// Additional validation for NVMe device name format
	deviceName := strings.TrimPrefix(device, "/dev/nvme")
	if len(deviceName) == 0 {
		return errors.New("invalid NVMe device name format")
	}

	// Security check: Validate device name length to prevent buffer overflow
	if len(deviceName) > 32 {
		return fmt.Errorf("NVMe device name exceeds maximum length of 32 characters (got %d)", len(deviceName))
	}

	// Security check: Validate characters in device name
	for i, char := range deviceName {
		if !isValidNVMeDeviceChar(char) {
			return fmt.Errorf("device path contains invalid character '%c' at position %d in device name", char, i)
		}
	}

	// Security check: Validate maximum path length to prevent buffer overflow attacks
	if len(device) > 255 {
		return fmt.Errorf("device path exceeds maximum allowed length of 255 characters (got %d)", len(device))
	}

	// Security check: Ensure the device name follows expected NVMe naming patterns
	if err := cfg.validateNVMeDeviceNamePattern(deviceName); err != nil {
		return fmt.Errorf("invalid NVMe device name pattern: %w", err)
	}

	// Log security-relevant validation if input was modified during sanitization
	if originalDevice != device {
		// This would be logged by the caller if they have access to a logger
		// We can't log here since we don't have access to a logger in the config validation
	}

	return nil
}

// isValidNVMeDeviceChar checks if a character is valid in an NVMe device name
func isValidNVMeDeviceChar(char rune) bool {
	// Allow digits, lowercase letters for NVMe device names
	// NVMe devices follow pattern: nvme<controller><namespace>[p<partition>]
	// Examples: nvme0n1, nvme1n2p1, nvme10n1p5
	return (char >= '0' && char <= '9') ||
		(char >= 'a' && char <= 'z') ||
		char == 'n' || char == 'p'
}

// validateNVMeDeviceNamePattern validates that the device name follows expected NVMe naming patterns
func (cfg *Config) validateNVMeDeviceNamePattern(deviceName string) error {
	// NVMe device names should follow pattern: <controller>n<namespace>[p<partition>]
	// Examples: 0n1, 1n2p1, 10n1p5

	if len(deviceName) < 3 {
		return errors.New("device name too short for valid NVMe pattern")
	}

	// Find the 'n' separator
	nIndex := strings.Index(deviceName, "n")
	if nIndex == -1 {
		return errors.New("device name must contain 'n' separator (controller<n>namespace)")
	}

	// Validate controller part (before 'n')
	controllerPart := deviceName[:nIndex]
	if len(controllerPart) == 0 {
		return errors.New("missing controller number")
	}
	for _, char := range controllerPart {
		if char < '0' || char > '9' {
			return fmt.Errorf("controller part contains non-digit character: %c", char)
		}
	}

	// Validate namespace and optional partition part (after 'n')
	remainingPart := deviceName[nIndex+1:]
	if len(remainingPart) == 0 {
		return errors.New("missing namespace number")
	}

	// Check for partition separator 'p'
	pIndex := strings.Index(remainingPart, "p")
	if pIndex == -1 {
		// No partition, validate entire remaining part as namespace
		for _, char := range remainingPart {
			if char < '0' || char > '9' {
				return fmt.Errorf("namespace part contains non-digit character: %c", char)
			}
		}
	} else {
		// Has partition, validate namespace part (before 'p')
		namespacePart := remainingPart[:pIndex]
		if len(namespacePart) == 0 {
			return errors.New("missing namespace number before partition")
		}
		for _, char := range namespacePart {
			if char < '0' || char > '9' {
				return fmt.Errorf("namespace part contains non-digit character: %c", char)
			}
		}

		// Validate partition part (after 'p')
		partitionPart := remainingPart[pIndex+1:]
		if len(partitionPart) == 0 {
			return errors.New("missing partition number after 'p'")
		}
		for _, char := range partitionPart {
			if char < '0' || char > '9' {
				return fmt.Errorf("partition part contains non-digit character: %c", char)
			}
		}

		// Security check: Ensure no additional 'p' separators
		if strings.Count(remainingPart, "p") > 1 {
			return errors.New("device name contains multiple partition separators")
		}
	}

	// Security check: Ensure no additional 'n' separators
	if strings.Count(deviceName, "n") > 1 {
		return errors.New("device name contains multiple namespace separators")
	}

	return nil
}
