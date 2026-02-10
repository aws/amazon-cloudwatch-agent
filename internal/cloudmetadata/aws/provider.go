// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws/session"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/ec2metadataprovider"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
)

// Provider implements the metadata provider interface for AWS.
// Directly uses ec2metadataprovider for IMDS access with retry and fallback support.
type Provider struct {
	logger   *zap.Logger
	metadata ec2metadataprovider.MetadataProvider

	// Cached metadata (fetched once at initialization)
	mu               sync.RWMutex
	instanceID       string
	instanceType     string
	imageID          string
	region           string
	availabilityZone string
	accountID        string
	hostname         string
	privateIP        string
	available        bool
}

// NewProvider creates a new AWS metadata provider
func NewProvider(ctx context.Context, logger *zap.Logger) (*Provider, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create AWS session
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create metadata provider with retry support
	metadataProvider := ec2metadataprovider.NewMetadataProvider(sess, retryer.GetDefaultRetryNumber())

	p := &Provider{
		logger:   logger,
		metadata: metadataProvider,
	}

	// Fetch initial metadata
	if err := p.fetchMetadata(ctx); err != nil {
		logger.Warn("Failed to fetch initial AWS metadata", zap.Error(err))
		// Don't return error - allow agent to start even if metadata unavailable
	}

	return p, nil
}

// fetchMetadata retrieves metadata from IMDS and caches it
func (p *Provider) fetchMetadata(ctx context.Context) error {
	// Fetch instance identity document (critical - must succeed)
	doc, err := p.metadata.Get(ctx)
	if err != nil {
		p.mu.Lock()
		p.available = false
		p.mu.Unlock()
		return fmt.Errorf("failed to get instance identity document: %w", err)
	}

	// Fetch hostname separately (optional - failure is acceptable)
	// Hostname is not critical for CloudWatch functionality
	hostname, err := p.metadata.Hostname(ctx)
	if err != nil {
		p.logger.Debug("Failed to fetch hostname", zap.Error(err))
		hostname = ""
	}

	// Cache all metadata
	p.mu.Lock()
	p.instanceID = doc.InstanceID
	p.instanceType = doc.InstanceType
	p.imageID = doc.ImageID
	p.region = doc.Region
	p.availabilityZone = doc.AvailabilityZone
	p.accountID = doc.AccountID
	p.privateIP = doc.PrivateIP
	p.hostname = hostname
	p.available = true // Available even if hostname is empty
	p.mu.Unlock()

	p.logger.Debug("[cloudmetadata/aws] Metadata fetched successfully",
		zap.String("region", doc.Region),
		zap.String("availabilityZone", doc.AvailabilityZone))

	return nil
}

// IsAWS detects if running on AWS by attempting to fetch metadata.
// This is used during cloud detection.
func IsAWS(ctx context.Context) bool {
	sess, err := session.NewSession()
	if err != nil {
		return false
	}

	metadataProvider := ec2metadataprovider.NewMetadataProvider(sess, retryer.GetDefaultRetryNumber())
	_, err = metadataProvider.Get(ctx)
	return err == nil
}

// GetInstanceID returns the EC2 instance ID.
func (p *Provider) GetInstanceID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.instanceID
}

// GetInstanceType returns the EC2 instance type
func (p *Provider) GetInstanceType() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.instanceType
}

// GetImageID returns the AMI ID
func (p *Provider) GetImageID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.imageID
}

// GetRegion returns the AWS region
func (p *Provider) GetRegion() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.region
}

// GetAvailabilityZone returns the availability zone
func (p *Provider) GetAvailabilityZone() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.availabilityZone
}

// GetAccountID returns the AWS account ID
func (p *Provider) GetAccountID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.accountID
}

// GetTags returns all EC2 tags.
// Note: EC2 tags require DescribeTags API calls, not IMDS.
// Use tagutil package for tag operations.
func (p *Provider) GetTags() map[string]string {
	return make(map[string]string)
}

// GetTag returns a specific EC2 tag value.
// Note: EC2 tags require DescribeTags API calls, not IMDS.
// Use tagutil package for tag operations.
func (p *Provider) GetTag(key string) (string, error) {
	return "", fmt.Errorf("EC2 tags not available via IMDS - use tagutil package")
}

// GetVolumeID returns the EBS volume ID for a given device name.
// Note: Volume mapping is handled by ec2tagger processor.
func (p *Provider) GetVolumeID(_ string) string {
	return ""
}

// GetScalingGroupName returns the Auto Scaling Group name.
// Note: ASG name requires DescribeTags API call, not IMDS.
// Use tagutil.GetAutoScalingGroupName() for ASG lookup.
func (p *Provider) GetScalingGroupName() string {
	return ""
}

// GetResourceGroupName returns empty string for AWS (Azure-specific concept)
func (p *Provider) GetResourceGroupName() string {
	return ""
}

// Refresh refreshes the metadata from IMDS
func (p *Provider) Refresh(ctx context.Context) error {
	return p.fetchMetadata(ctx)
}

// IsAvailable returns true if EC2 metadata is available.
// This checks if the provider has successfully fetched instance metadata.
func (p *Provider) IsAvailable() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.available
}

// GetHostname returns the EC2 instance hostname
func (p *Provider) GetHostname() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.hostname
}

// GetPrivateIP returns the EC2 instance private IP address
func (p *Provider) GetPrivateIP() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.privateIP
}

// GetCloudProvider returns the cloud provider type.
// Returns 1 (CloudProviderAWS from internal/cloudmetadata/constants.go).
// NOTE: Cannot import cloudmetadata package here due to import cycle.
func (p *Provider) GetCloudProvider() int {
	return 1 // Must match cloudmetadata.CloudProviderAWS
}
