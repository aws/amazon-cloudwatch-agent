// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

// MockEC2TagValue sets up a mock for GetEC2TagValue function for testing
func MockEC2TagValue(mockFunc func(string) string) func() {
	original := getEC2TagValueFunc
	getEC2TagValueFunc = mockFunc

	// Return cleanup function
	return func() {
		getEC2TagValueFunc = original
	}
}

// MockEC2TagValueWithMap sets up a mock that returns values from a map
func MockEC2TagValueWithMap(tagMap map[string]string) func() {
	return MockEC2TagValue(func(tagKey string) string {
		if value, exists := tagMap[tagKey]; exists {
			return value
		}
		return ""
	})
}

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
	tagsCleanup := MockEC2TagValueWithMap(tagMap)

	// Return combined cleanup function
	return func() {
		metadataCleanup()
		tagsCleanup()
	}
}
