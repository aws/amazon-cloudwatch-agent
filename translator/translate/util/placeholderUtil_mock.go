// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
)

// MockEC2MetadataInfoProvider sets up a mock for the EC2 metadata provider
func MockEC2MetadataInfoProvider(mockFunc func() *Metadata) func() {
	original := ec2MetadataInfoProviderFunc
	ec2MetadataInfoProviderFunc = mockFunc

	// Return cleanup function
	return func() {
		ec2MetadataInfoProviderFunc = original
	}
}

// MockEC2MetadataInfoProviderWithValues sets up a mock with specific metadata values
func MockEC2MetadataInfoProviderWithValues(instanceID, instanceType, imageID, hostname, privateIP, accountID string) func() {
	return MockEC2MetadataInfoProvider(func() *Metadata {
		return &Metadata{
			InstanceID:   instanceID,
			InstanceType: instanceType,
			ImageID:      imageID,
			Hostname:     hostname,
			PrivateIP:    privateIP,
			AccountID:    accountID,
		}
	})
}

// MockTagMetadataProvider sets up a mock for the tag metadata provider
func MockTagMetadataProvider(mockFunc func() map[string]string) func() {
	original := tagMetadataProvider
	tagMetadataProvider = mockFunc

	// Return cleanup function
	return func() {
		tagMetadataProvider = original
	}
}

// MockTagMetadataProviderWithMap sets up a mock that returns values from a map
func MockTagMetadataProviderWithMap(tagMap map[string]string) func() {
	return MockTagMetadataProvider(func() map[string]string {
		result := make(map[string]string)
		for k, v := range tagMap {
			// Map tag keys to their append_dimensions equivalents
			switch k {
			case "aws:autoscaling:groupName":
				result[ec2tagger.SupportedAppendDimensions["AutoScalingGroupName"]] = v
			default:
				// For other tags, use them as-is
				result[k] = v
			}
		}
		return result
	})
}

// MockAWSMetadata represents the AWS metadata values for mocking
type MockAWSMetadata struct {
	InstanceID   string
	InstanceType string
	ImageID      string
	Hostname     string
	PrivateIP    string
	AccountID    string
}

// MockCompleteAWSMetadata sets up mocks for both EC2 metadata and EC2 tags using a struct
func MockCompleteAWSMetadata(metadata MockAWSMetadata, tagMap map[string]string) func() {
	// Setup EC2 metadata mock
	metadataCleanup := MockEC2MetadataInfoProviderWithValues(
		metadata.InstanceID,
		metadata.InstanceType,
		metadata.ImageID,
		metadata.Hostname,
		metadata.PrivateIP,
		metadata.AccountID,
	)

	// Setup EC2 tags mock
	tagsCleanup := MockTagMetadataProviderWithMap(tagMap)

	// Return combined cleanup function
	return func() {
		metadataCleanup()
		tagsCleanup()
	}
}
