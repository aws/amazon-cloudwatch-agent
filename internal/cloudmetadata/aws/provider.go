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
func NewProvider(_ context.Context, logger *zap.Logger) (*Provider, error) {
	// Initialize EC2 util singleton
	_ = ec2util.GetEC2UtilSingleton()

	return &Provider{
		logger: logger,
	}, nil
}

// IsAWS detects if running on AWS by checking for EC2 metadata availability
func IsAWS(_ context.Context) bool {
	ec2 := ec2util.GetEC2UtilSingleton()
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
	// EC2 util does not expose availability zone
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
func (p *Provider) GetTags() map[string]string {
	// EC2 tags are fetched on-demand via tagutil for supported keys
	return make(map[string]string)
}

// GetTag returns a specific EC2 tag value
// Supports AutoScalingGroupName via existing tagutil integration
func (p *Provider) GetTag(key string) (string, error) {
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
func (p *Provider) GetVolumeID(_ string) string {
	// Volume mapping is handled by ec2tagger processor
	return ""
}

// GetScalingGroupName returns the Auto Scaling Group name
func (p *Provider) GetScalingGroupName() string {
	asgName, _ := p.GetTag("AutoScalingGroupName")
	return asgName
}

// Refresh refreshes the metadata
func (p *Provider) Refresh(_ context.Context) error {
	// EC2 metadata is fetched once at startup via ec2util singleton
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
// NOTE: Duplicated from internal/cloudmetadata/mask.go to avoid import cycle
// (aws → cloudmetadata → factory → aws).
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
// NOTE: Duplicated from internal/cloudmetadata/mask.go to avoid import cycle.
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
