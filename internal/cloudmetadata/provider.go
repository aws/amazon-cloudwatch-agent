// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudmetadata

import (
	"context"
)

// CloudProvider represents the cloud platform
type CloudProvider int

const (
	CloudProviderUnknown CloudProvider = iota
	CloudProviderAWS
	CloudProviderAzure
)

// String returns the string representation of the cloud provider
func (c CloudProvider) String() string {
	switch c {
	case CloudProviderAWS:
		return "AWS"
	case CloudProviderAzure:
		return "Azure"
	default:
		return "Unknown"
	}
}

// Provider is a cloud-agnostic interface for fetching instance metadata
type Provider interface {
	// GetInstanceID returns the instance/VM ID
	GetInstanceID() string

	// GetInstanceType returns the instance/VM size/type
	GetInstanceType() string

	// GetImageID returns the image/AMI ID
	GetImageID() string

	// GetRegion returns the region/location
	GetRegion() string

	// GetAvailabilityZone returns the availability zone (AWS) or zone (Azure)
	GetAvailabilityZone() string

	// GetAccountID returns the account ID (AWS) or subscription ID (Azure)
	GetAccountID() string

	// GetHostname returns the hostname of the instance
	GetHostname() string

	// GetPrivateIP returns the private IP address of the instance
	GetPrivateIP() string

	// GetCloudProvider returns the cloud provider type as int
	// Use CloudProviderAWS, CloudProviderAzure constants to compare
	GetCloudProvider() int

	// GetTags returns all tags as a map
	GetTags() map[string]string

	// GetTag returns a specific tag value
	GetTag(key string) (string, error)

	// GetVolumeID returns the volume/disk ID for a given device name
	// Returns empty string if not found
	GetVolumeID(deviceName string) string

	// GetScalingGroupName returns the Auto Scaling Group name (AWS) or VM Scale Set name (Azure)
	GetScalingGroupName() string

	// Refresh fetches the latest metadata from the cloud provider
	Refresh(ctx context.Context) error

	// IsAvailable returns true if metadata is available
	IsAvailable() bool
}
