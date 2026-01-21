// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudmetadata

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata/aws"
	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata/azure"
)

// DetectCloudProvider attempts to detect the cloud provider
// Returns CloudProviderUnknown if detection fails
func DetectCloudProvider(ctx context.Context, logger *zap.Logger) CloudProvider {
	// Try Azure first (faster detection via DMI)
	if azure.IsAzure() {
		logger.Info("Detected cloud provider: Azure")
		return CloudProviderAzure
	}

	// Try AWS
	if aws.IsAWS(ctx) {
		logger.Info("Detected cloud provider: AWS")
		return CloudProviderAWS
	}

	logger.Warn("Could not detect cloud provider, defaulting to AWS for backward compatibility")
	return CloudProviderAWS // Default to AWS for backward compatibility
}

// NewProvider creates a new metadata provider for the detected cloud
func NewProvider(ctx context.Context, logger *zap.Logger) (Provider, error) {
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
