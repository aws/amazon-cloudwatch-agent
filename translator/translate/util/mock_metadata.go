// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
)

// SetupMockMetadataForTesting sets up mock metadata service for testing
// This function can be called from other test packages to mock AWS metadata
func SetupMockMetadataForTesting() func() {
	// Store original functions
	originalGetEC2Metadata := getEC2Metadata
	originalGetEC2TagValue := getEC2TagValue

	// Setup mock metadata service with test data
	getEC2Metadata = func() (ec2metadata.EC2InstanceIdentityDocument, error) {
		return ec2metadata.EC2InstanceIdentityDocument{
			InstanceID:   "i-1234567890abcdef0",
			InstanceType: "t3.medium",
			ImageID:      "ami-0abcdef1234567890",
			Region:       "us-west-2",
		}, nil
	}

	getEC2TagValue = func(_, _, tagKey string) string {
		mockTags := map[string]string{
			"AutoScalingGroupName": "production-web-asg",
		}
		return mockTags[tagKey]
	}

	// Return cleanup function
	return func() {
		getEC2Metadata = originalGetEC2Metadata
		getEC2TagValue = originalGetEC2TagValue
	}
}
