// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/tagutil"
)

// CloudProviderAWS is the constant for AWS cloud provider (matches cloudmetadata.CloudProviderAWS)
const CloudProviderAWS = 1

// Provider implements the metadata provider interface for AWS
type Provider struct {
	logger *zap.Logger
}

// NewProvider creates a new AWS metadata provider
func NewProvider(ctx context.Context, logger *zap.Logger) (*Provider, error) {
	// Initialize EC2 util singleton
	_ = ec2util.GetEC2UtilSingleton()

	return &Provider{
		logger: logger,
	}, nil
}

// IsAWS detects if running on AWS
// This is a simple check - more sophisticated detection can be added
func IsAWS(ctx context.Context) bool {
	ec2 := ec2util.GetEC2UtilSingleton()
	// If we have a region, we're likely on AWS
	return ec2.Region != ""
}

// GetInstanceID returns the EC2 instance ID
func (p *Provider) GetInstanceID() string {
	value := ec2util.GetEC2UtilSingleton().InstanceID
	p.logger.Debug("[cloudmetadata/aws] GetInstanceID called",
		zap.String("value", maskValue(value)))
	return value
}

// GetInstanceType returns the EC2 instance type
func (p *Provider) GetInstanceType() string {
	value := ec2util.GetEC2UtilSingleton().InstanceType
	p.logger.Debug("[cloudmetadata/aws] GetInstanceType called",
		zap.String("value", value))
	return value
}

// GetImageID returns the AMI ID
func (p *Provider) GetImageID() string {
	value := ec2util.GetEC2UtilSingleton().ImageID
	p.logger.Debug("[cloudmetadata/aws] GetImageID called",
		zap.String("value", maskValue(value)))
	return value
}

// GetRegion returns the AWS region
func (p *Provider) GetRegion() string {
	value := ec2util.GetEC2UtilSingleton().Region
	p.logger.Debug("[cloudmetadata/aws] GetRegion called",
		zap.String("value", value))
	return value
}

// GetAvailabilityZone returns the availability zone
func (p *Provider) GetAvailabilityZone() string {
	// Not directly available in ec2util, return empty for now
	// Can be added if needed
	p.logger.Debug("[cloudmetadata/aws] GetAvailabilityZone called (not implemented)")
	return ""
}

// GetAccountID returns the AWS account ID
func (p *Provider) GetAccountID() string {
	value := ec2util.GetEC2UtilSingleton().AccountID
	p.logger.Debug("[cloudmetadata/aws] GetAccountID called",
		zap.String("value", maskValue(value)))
	return value
}

// GetTags returns all EC2 tags
// Note: This requires EC2 API call and proper IAM permissions
func (p *Provider) GetTags() map[string]string {
	// EC2 tags are fetched on-demand via tagutil
	// Return empty map for now - tags are typically accessed via GetTag
	return make(map[string]string)
}

// GetTag returns a specific EC2 tag value
func (p *Provider) GetTag(key string) (string, error) {
	// Use existing tagutil functionality
	// Note: This only supports specific tags like AutoScalingGroupName
	if key == "aws:autoscaling:groupName" || key == "AutoScalingGroupName" {
		instanceID := ec2util.GetEC2UtilSingleton().InstanceID
		asgName := tagutil.GetAutoScalingGroupName(instanceID)
		if asgName == "" {
			return "", fmt.Errorf("tag %s not found", key)
		}
		return asgName, nil
	}

	return "", fmt.Errorf("tag %s not supported", key)
}

// GetVolumeID returns the EBS volume ID for a given device name
// This requires EC2 DescribeVolumes API call
func (p *Provider) GetVolumeID(deviceName string) string {
	// Volume mapping is handled by ec2tagger processor
	// Return empty string here - actual implementation is in ec2tagger
	return ""
}

// GetScalingGroupName returns the Auto Scaling Group name
func (p *Provider) GetScalingGroupName() string {
	asgName, _ := p.GetTag("AutoScalingGroupName")
	return asgName
}

// Refresh refreshes the metadata
// For AWS, ec2util is a singleton that fetches once at startup
// No refresh mechanism currently
func (p *Provider) Refresh(ctx context.Context) error {
	// EC2 metadata is fetched once at startup via ec2util singleton
	// No refresh mechanism in current implementation
	return nil
}

// IsAvailable returns true if EC2 metadata is available
func (p *Provider) IsAvailable() bool {
	return ec2util.GetEC2UtilSingleton().InstanceID != ""
}

// GetHostname returns the EC2 instance hostname
func (p *Provider) GetHostname() string {
	value := ec2util.GetEC2UtilSingleton().Hostname
	p.logger.Debug("[cloudmetadata/aws] GetHostname called",
		zap.String("value", value))
	return value
}

// GetPrivateIP returns the EC2 instance private IP address
func (p *Provider) GetPrivateIP() string {
	value := ec2util.GetEC2UtilSingleton().PrivateIP
	p.logger.Debug("[cloudmetadata/aws] GetPrivateIP called",
		zap.String("value", maskIPAddress(value)))
	return value
}

// GetCloudProvider returns the cloud provider type (AWS = 1)
func (p *Provider) GetCloudProvider() int {
	return CloudProviderAWS
}

// maskValue masks sensitive values for logging
func maskValue(value string) string {
	if value == "" {
		return "<empty>"
	}
	if len(value) <= 4 {
		return "<present>"
	}
	return value[:4] + "..."
}

// maskIPAddress masks IP addresses for logging (e.g., 10.0.x.x)
func maskIPAddress(ip string) string {
	if ip == "" {
		return "<empty>"
	}
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".x.x"
	}
	return "<present>"
}
