// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsinstancestorenvmereceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/aws/amazon-cloudwatch-agent/receiver/awsinstancestorenvmereceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg)

	config, ok := cfg.(*Config)
	assert.True(t, ok)
	assert.Empty(t, config.Devices)

	// Verify default configuration structure
	assert.NotNil(t, config.ControllerConfig)
	assert.NotNil(t, config.MetricsBuilderConfig)

	// Verify configuration is valid
	assert.NoError(t, config.Validate())
}

func TestCreateMetricsReceiver(t *testing.T) {
	testCases := []struct {
		name    string
		devices []string
	}{
		{
			name:    "no devices",
			devices: []string{},
		},
		{
			name:    "with devices",
			devices: []string{"/dev/nvme0n1", "/dev/nvme1n1"},
		},
		{
			name:    "with wildcard",
			devices: []string{"*"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Devices = tc.devices

			receiver, err := createMetricsReceiver(
				context.Background(),
				receivertest.NewNopSettings(component.MustNewType("awsinstancestorenvmereceiver")),
				cfg,
				consumertest.NewNop(),
			)

			require.NoError(t, err)
			require.NotNil(t, receiver)
		})
	}
}

func TestCreateMetricsReceiverWithInvalidConfig(t *testing.T) {
	testCases := []struct {
		name         string
		modifyConfig func(*Config)
		expectError  bool
	}{
		{
			name: "invalid device path",
			modifyConfig: func(cfg *Config) {
				cfg.Devices = []string{"/invalid/path"}
			},
			expectError: false, // Config validation happens at receiver level, not factory level
		},
		{
			name: "empty device path",
			modifyConfig: func(cfg *Config) {
				cfg.Devices = []string{""}
			},
			expectError: false, // Config validation happens at receiver level, not factory level
		},
		{
			name: "invalid controller config",
			modifyConfig: func(cfg *Config) {
				cfg.ControllerConfig = scraperhelper.ControllerConfig{
					CollectionInterval: -1, // Invalid negative interval
				}
			},
			expectError: false, // Config validation happens at receiver level, not factory level
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			tc.modifyConfig(cfg)

			receiver, err := createMetricsReceiver(
				context.Background(),
				receivertest.NewNopSettings(component.MustNewType("awsinstancestorenvmereceiver")),
				cfg,
				consumertest.NewNop(),
			)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, receiver)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, receiver)
			}
		})
	}
}

func TestFactoryType(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestFactoryCreateMetricsReceiverWithNilConsumer(t *testing.T) {
	cfg := createDefaultConfig()

	receiver, err := createMetricsReceiver(
		context.Background(),
		receivertest.NewNopSettings(component.MustNewType("awsinstancestorenvmereceiver")),
		cfg,
		nil, // nil consumer should be handled gracefully
	)

	// The scraperhelper should handle nil consumer gracefully
	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}

func TestFactoryCreateMetricsReceiverWithWrongConfigType(t *testing.T) {
	// This test verifies that the factory panics with wrong config types
	// In practice, this should not happen due to the collector framework, but we test for robustness
	defer func() {
		if r := recover(); r != nil {
			// Expected to panic due to type assertion
			// The panic should be a type assertion error
			assert.NotNil(t, r)
		} else {
			t.Fatal("Expected panic due to wrong config type, but no panic occurred")
		}
	}()

	// Pass wrong config type - this should panic during type assertion
	_, _ = createMetricsReceiver(
		context.Background(),
		receivertest.NewNopSettings(component.MustNewType("awsinstancestorenvmereceiver")),
		&struct{}{}, // Wrong config type
		consumertest.NewNop(),
	)
}

func TestFactoryCreateMetricsReceiverErrorHandling(t *testing.T) {
	// Test that the factory properly handles various edge cases
	testCases := []struct {
		name           string
		setupConfig    func() *Config
		expectError    bool
		errorSubstring string
	}{
		{
			name: "valid config with empty devices",
			setupConfig: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Devices = []string{}
				return cfg
			},
			expectError: false,
		},
		{
			name: "valid config with wildcard",
			setupConfig: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Devices = []string{"*"}
				return cfg
			},
			expectError: false,
		},
		{
			name: "valid config with specific devices",
			setupConfig: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Devices = []string{"/dev/nvme0n1", "/dev/nvme1n1"}
				return cfg
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.setupConfig()

			receiver, err := createMetricsReceiver(
				context.Background(),
				receivertest.NewNopSettings(component.MustNewType("awsinstancestorenvmereceiver")),
				cfg,
				consumertest.NewNop(),
			)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorSubstring != "" {
					assert.Contains(t, err.Error(), tc.errorSubstring)
				}
				assert.Nil(t, receiver)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, receiver)
			}
		})
	}
}
