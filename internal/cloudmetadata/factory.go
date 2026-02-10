// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudmetadata

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata/azure"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorctx "github.com/aws/amazon-cloudwatch-agent/translator/context"
)

const (
	networkRetries    = 5
	networkRetrySleep = 1 * time.Second
)

// isOnPremMode checks if agent is running in on-premises mode (same as original ec2util)
func isOnPremMode() bool {
	mode := translatorctx.CurrentContext().Mode()
	return mode == config.ModeOnPrem || mode == config.ModeOnPremise
}

// DetectCloudProvider attempts to detect the cloud provider.
// Assumes caller has already checked for onPrem mode.
// Detection order:
// 1. Azure DMI/IMDS detection (fast local checks first)
// 2. AWS IMDS detection
func DetectCloudProvider(ctx context.Context, logger *zap.Logger) CloudProvider {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Try Azure first (faster detection via DMI, then IMDS)
	if azure.IsAzure() {
		logger.Info("Detected cloud provider: Azure")
		return CloudProviderAzure
	}

	// Try AWS
	if aws.IsAWS(ctx) {
		logger.Info("Detected cloud provider: AWS")
		return CloudProviderAWS
	}

	logger.Warn("Could not detect cloud provider")
	return CloudProviderUnknown
}

// NewProvider creates a new metadata provider for the detected cloud.
// Waits for network to be available before detection (matches original ec2util behavior).
// Returns nil for onPrem mode (no cloud metadata needed).
func NewProvider(ctx context.Context, logger *zap.Logger) (Provider, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Check onPrem mode once, early (matches original ec2util)
	if isOnPremMode() {
		logger.Info("OnPrem mode - skipping cloud metadata initialization")
		return nil, fmt.Errorf("onprem mode: cloud metadata not needed")
	}

	// Wait for network interface to be up (same as original ec2util)
	waitForNetwork(ctx, logger)

	// Detect cloud provider (assumes onPrem already checked)
	cloudProvider := DetectCloudProvider(ctx, logger)

	switch cloudProvider {
	case CloudProviderAWS:
		return aws.NewProvider(ctx, logger)
	case CloudProviderAzure:
		return azure.NewProvider(ctx, logger)
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %v", cloudProvider)
	}
}

// waitForNetwork waits for a non-loopback network interface to be up.
// Matches original ec2util behavior: 5 retries with 1s sleep.
func waitForNetwork(ctx context.Context, logger *zap.Logger) {
	for retry := 0; retry < networkRetries; retry++ {
		select {
		case <-ctx.Done():
			logger.Warn("Network wait cancelled", zap.Error(ctx.Err()))
			return
		default:
		}

		ifs, err := net.Interfaces()
		if err != nil {
			logger.Error("Failed to fetch network interfaces", zap.Error(err))
			continue
		}

		for _, iface := range ifs {
			if (iface.Flags&net.FlagUp) != 0 && (iface.Flags&net.FlagLoopback) == 0 {
				logger.Debug("Found active network interface", zap.String("name", iface.Name))
				return
			}
		}

		logger.Warn("Waiting for network to be up", zap.Int("retry", retry+1), zap.Int("maxRetries", networkRetries))

		// Sleep with context awareness
		select {
		case <-time.After(networkRetrySleep):
		case <-ctx.Done():
			logger.Warn("Network wait cancelled during sleep", zap.Error(ctx.Err()))
			return
		}
	}

	logger.Warn("Network wait exhausted all retries", zap.Int("retries", networkRetries))
}
