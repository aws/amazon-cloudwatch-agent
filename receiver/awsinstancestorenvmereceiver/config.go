// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsinstancestorenvmereceiver

import (
	"errors"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/aws/amazon-cloudwatch-agent/receiver/awsinstancestorenvmereceiver/internal/metadata"
)

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

// validateDevice validates a single device path
func (cfg *Config) validateDevice(device string) error {
	if device == "" {
		return errors.New("device path cannot be empty")
	}

	// Support wildcard for auto-discovery
	if device == "*" {
		return nil
	}

	// Check for path traversal attempts first
	if strings.Contains(device, "..") {
		return errors.New("device path cannot contain '..'")
	}

	// Validate device path format
	if !strings.HasPrefix(device, "/dev/") {
		return errors.New("device path must start with /dev/")
	}

	// Prevent directory traversal attacks
	cleanPath := filepath.Clean(device)
	if cleanPath != device {
		return errors.New("device path contains invalid characters")
	}

	// Validate NVMe device naming pattern
	if !strings.HasPrefix(device, "/dev/nvme") {
		return errors.New("device path must be an NVMe device (/dev/nvme*)")
	}

	return nil
}
