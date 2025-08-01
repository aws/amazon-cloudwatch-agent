// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsnvmereceiver

import (
	"errors"
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

	// Sanitize input by trimming whitespace and null bytes
	device = strings.TrimSpace(device)
	if strings.Contains(device, "\x00") {
		return errors.New("device path cannot contain null bytes")
	}

	// Validate device path format first
	if !strings.HasPrefix(device, "/dev/") {
		return errors.New("device path must start with /dev/")
	}

	// Check for path traversal attempts - multiple patterns
	if strings.Contains(device, "..") {
		return errors.New("device path cannot contain '..'")
	}
	if strings.Contains(device, "./") {
		return errors.New("device path cannot contain relative path components")
	}
	if strings.Contains(device, "//") {
		return errors.New("device path cannot contain double slashes")
	}

	// Prevent directory traversal attacks
	cleanPath := filepath.Clean(device)
	if cleanPath != device {
		return errors.New("device path contains invalid characters")
	}

	// Validate absolute path doesn't escape /dev
	absPath, err := filepath.Abs(device)
	if err != nil {
		return errors.New("device path is not a valid absolute path")
	}
	if !strings.HasPrefix(absPath, "/dev/") {
		return errors.New("device path must resolve to /dev/ directory")
	}

	// Validate NVMe device naming pattern with stricter regex-like validation
	if !strings.HasPrefix(device, "/dev/nvme") {
		return errors.New("device path must be an NVMe device (/dev/nvme*)")
	}

	// Additional validation for NVMe device name format
	deviceName := strings.TrimPrefix(device, "/dev/nvme")
	if len(deviceName) == 0 {
		return errors.New("invalid NVMe device name format")
	}

	// Check for suspicious characters that could be used in attacks
	for _, char := range deviceName {
		if !isValidNVMeDeviceChar(char) {
			return errors.New("device path contains invalid characters for NVMe device")
		}
	}

	// Validate maximum path length to prevent buffer overflow attacks
	if len(device) > 255 {
		return errors.New("device path exceeds maximum allowed length")
	}

	return nil
}

// isValidNVMeDeviceChar checks if a character is valid in an NVMe device name
func isValidNVMeDeviceChar(char rune) bool {
	// Allow digits, lowercase letters, and 'n' and 'p' for namespace and partition
	return (char >= '0' && char <= '9') ||
		(char >= 'a' && char <= 'z') ||
		char == 'n' || char == 'p'
}
