// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/tagutil"
)

const (
	dummyInstanceId = "some_instance_id"
	dummyHostName   = "some_hostname"
	dummyPrivateIp  = "some_private_ip"
	dummyAccountId  = "some_account_id"
)

func TestHostName(t *testing.T) {
	assert.True(t, getHostName() != unknownHostname)
}

func TestIpAddress(t *testing.T) {
	assert.True(t, getIpAddress() != unknownIPAddress)
}

func TestGetMetadataInfo(t *testing.T) {
	m := GetMetadataInfo(mockMetadataProvider(dummyInstanceId, dummyHostName, dummyPrivateIp, dummyAccountId))
	assert.Equal(t, dummyInstanceId, m[instanceIdPlaceholder])
	assert.Equal(t, dummyHostName, m[hostnamePlaceholder])
	assert.Equal(t, dummyPrivateIp, m[ipAddressPlaceholder])
	assert.Equal(t, dummyAccountId, m[accountIdPlaceholder])
}

func TestGetMetadataInfoEmptyInstanceId(t *testing.T) {
	m := GetMetadataInfo(mockMetadataProvider("", dummyHostName, dummyPrivateIp, dummyAccountId))
	assert.Equal(t, unknownInstanceID, m[instanceIdPlaceholder])
}

func TestGetMetadataInfoUsesLocalHostname(t *testing.T) {
	m := GetMetadataInfo(mockMetadataProvider(dummyInstanceId, "", dummyPrivateIp, dummyAccountId))
	assert.Equal(t, getHostName(), m[hostnamePlaceholder])
}

func TestGetMetadataInfoDerivesIpAddress(t *testing.T) {
	m := GetMetadataInfo(mockMetadataProvider(dummyInstanceId, dummyHostName, "", dummyAccountId))
	assert.Equal(t, getIpAddress(), m[ipAddressPlaceholder])
}

func TestGetMetadataInfoEmptyAccountId(t *testing.T) {
	m := GetMetadataInfo(mockMetadataProvider(dummyInstanceId, dummyHostName, dummyPrivateIp, ""))
	assert.Equal(t, unknownAccountID, m[accountIdPlaceholder])
}

func mockMetadataProvider(instanceId, hostname, privateIp, accountId string) func() *Metadata {
	return func() *Metadata {
		return &Metadata{
			InstanceID: instanceId,
			Hostname:   hostname,
			PrivateIP:  privateIp,
			AccountID:  accountId,
		}
	}
}
func TestResolveAWSMetadataPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "No AWS placeholders",
			input: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "Unresolved AWS placeholder should be omitted",
			input: map[string]interface{}{
				"InstanceType":         "t3.medium",
				"AutoScalingGroupName": "${aws:AutoScalingGroupName}",
				"ImageId":              "ami-12345",
			},
			expected: map[string]interface{}{
				"InstanceType": "t3.medium",
				"ImageId":      "ami-12345",
			},
		},
		{
			name: "Mixed resolved and unresolved placeholders",
			input: map[string]interface{}{
				"InstanceType": "${aws:InstanceType}",
				"ImageId":      "${aws:ImageId}",
				"RegularKey":   "regular_value",
			},
			expected: map[string]interface{}{
				"InstanceType": unknownInstanceType, // Should be resolved to default
				"ImageId":      unknownImageID,      // Should be resolved to default
				"RegularKey":   "regular_value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveAWSMetadataPlaceholders(tt.input)
			resultMap := result.(map[string]interface{})

			// Check that expected keys are present with correct values
			for k, v := range tt.expected {
				assert.Equal(t, v, resultMap[k], "Key %s should have value %v", k, v)
			}

			// Check that no unexpected keys are present
			assert.Equal(t, len(tt.expected), len(resultMap), "Result should have exactly %d keys", len(tt.expected))
		})
	}
}
func TestResolveAWSMetadataPlaceholdersWithMockedData(t *testing.T) {
	// Reset cache before test
	tagutil.ResetTagsCache()

	// Mock the metadata provider for this test
	originalProvider := Ec2MetadataInfoProvider
	Ec2MetadataInfoProvider = func() *Metadata {
		return &Metadata{
			InstanceID:   "i-1234567890abcdef0",
			InstanceType: "t3.large",
			ImageID:      "ami-0abcdef1234567890",
			Hostname:     "test-hostname",
			PrivateIP:    "10.0.1.100",
			AccountID:    "123456789012",
		}
	}

	// Mock the tag metadata provider for this test
	originalTagProvider := tagMetadataProvider
	tagMetadataProvider = func() map[string]string {
		return map[string]string{
			ec2tagger.SupportedAppendDimensions["AutoScalingGroupName"]: "my-test-asg",
		}
	}

	defer func() {
		Ec2MetadataInfoProvider = originalProvider
		tagMetadataProvider = originalTagProvider
		tagutil.ResetTagsCache()
	}()

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "All AWS placeholders resolved successfully",
			input: map[string]interface{}{
				"InstanceType":         "${aws:InstanceType}",
				"AutoScalingGroupName": "${aws:AutoScalingGroupName}",
				"ImageId":              "${aws:ImageId}",
				"InstanceId":           "${aws:InstanceId}",
				"RegularKey":           "regular_value",
			},
			expected: map[string]interface{}{
				"InstanceType":         "t3.large",
				"AutoScalingGroupName": "my-test-asg",
				"ImageId":              "ami-0abcdef1234567890",
				"InstanceId":           "i-1234567890abcdef0",
				"RegularKey":           "regular_value",
			},
		},
		{
			name: "Mixed AWS placeholders with some unresolvable",
			input: map[string]interface{}{
				"InstanceType":         "${aws:InstanceType}",
				"AutoScalingGroupName": "${aws:AutoScalingGroupName}",
				"UnknownPlaceholder":   "${aws:SomeUnknownValue}",
				"RegularKey":           "regular_value",
			},
			expected: map[string]interface{}{
				"InstanceType":         "t3.large",
				"AutoScalingGroupName": "my-test-asg",
				"RegularKey":           "regular_value",
				// UnknownPlaceholder should be omitted
			},
		},
		{
			name: "Ensure we do not resolve non-aws placeholders",
			input: map[string]interface{}{
				"InstanceId": "{instance_id}",
				"Hostname":   "{hostname}",
				"RegularKey": "regular_value",
			},
			expected: map[string]interface{}{
				"InstanceId": "{instance_id}",
				"Hostname":   "{hostname}",
				"RegularKey": "regular_value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveAWSMetadataPlaceholders(tt.input)
			resultMap := result.(map[string]interface{})

			// Check that expected keys are present with correct values
			for k, v := range tt.expected {
				assert.Equal(t, v, resultMap[k], "Key %s should have value %v", k, v)
			}

			// Check that no unexpected keys are present
			assert.Equal(t, len(tt.expected), len(resultMap), "Result should have exactly %d keys", len(tt.expected))
		})
	}
}
func TestAWSMetadataFunctionality(t *testing.T) {
	// Test that AWS metadata placeholders are resolved correctly
	// Note: We rely on ec2util.GetEC2UtilSingleton() for caching, not additional layers

	originalProvider := Ec2MetadataInfoProvider
	Ec2MetadataInfoProvider = func() *Metadata {
		return &Metadata{
			InstanceID:   "i-test123",
			InstanceType: "t3.micro",
			ImageID:      "ami-test123",
		}
	}
	defer func() {
		Ec2MetadataInfoProvider = originalProvider
	}()

	// Test single placeholder resolution
	input1 := map[string]interface{}{
		"InstanceId": "${aws:InstanceId}",
	}
	result1 := ResolveAWSMetadataPlaceholders(input1)
	resultMap1 := result1.(map[string]interface{})
	assert.Equal(t, "i-test123", resultMap1["InstanceId"])

	// Test multiple placeholder resolution
	input2 := map[string]interface{}{
		"InstanceId":   "${aws:InstanceId}",
		"InstanceType": "${aws:InstanceType}",
		"ImageId":      "${aws:ImageId}",
	}
	result2 := ResolveAWSMetadataPlaceholders(input2)
	resultMap2 := result2.(map[string]interface{})
	assert.Equal(t, "i-test123", resultMap2["InstanceId"])
	assert.Equal(t, "t3.micro", resultMap2["InstanceType"])
	assert.Equal(t, "ami-test123", resultMap2["ImageId"])
}

// --- Cloudmetadata Singleton Integration Tests ---

func TestGetMetadataInfo_WithCloudmetadataSingleton(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	mock := &cloudmetadata.MockProvider{
		InstanceID: "i-singleton123",
		Region:     "us-west-2",
		Hostname:   "singleton-host",
		PrivateIP:  "192.168.1.1",
		AccountID:  "999888777666",
	}
	cloudmetadata.SetGlobalProviderForTest(mock)

	result := GetMetadataInfo(nil)

	assert.Equal(t, "i-singleton123", result[instanceIdPlaceholder])
	assert.Equal(t, "us-west-2", result[awsRegionPlaceholder])
	assert.Equal(t, "singleton-host", result[hostnamePlaceholder])
	assert.Equal(t, "192.168.1.1", result[ipAddressPlaceholder])
	assert.Equal(t, "999888777666", result[accountIdPlaceholder])
}

func TestGetMetadataInfo_FallbackToLegacy(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	// Don't set singleton - test fallback

	legacyMock := mockMetadataProvider("i-legacy456", "legacy-host", "10.0.0.99", "111222333444")

	result := GetMetadataInfo(legacyMock)

	assert.Equal(t, "i-legacy456", result[instanceIdPlaceholder])
	assert.Equal(t, "legacy-host", result[hostnamePlaceholder])
	assert.Equal(t, "10.0.0.99", result[ipAddressPlaceholder])
	assert.Equal(t, "111222333444", result[accountIdPlaceholder])
}

func TestGetMetadataInfo_SingletonTakesPrecedence(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	// Set singleton
	singletonMock := &cloudmetadata.MockProvider{
		InstanceID: "i-singleton",
		Region:     "singleton-region",
		Hostname:   "singleton-host",
		PrivateIP:  "10.1.1.1",
		AccountID:  "singleton-account",
	}
	cloudmetadata.SetGlobalProviderForTest(singletonMock)

	// Also provide legacy (should be ignored)
	legacyMock := mockMetadataProvider("i-legacy", "legacy-host", "10.2.2.2", "legacy-account")

	result := GetMetadataInfo(legacyMock)

	// Singleton should win
	assert.Equal(t, "i-singleton", result[instanceIdPlaceholder])
	assert.Equal(t, "singleton-region", result[awsRegionPlaceholder])
	assert.Equal(t, "singleton-host", result[hostnamePlaceholder])
	assert.Equal(t, "10.1.1.1", result[ipAddressPlaceholder])
	assert.Equal(t, "singleton-account", result[accountIdPlaceholder])
}

func TestGetMetadataInfo_SingletonWithEmptyPrivateIP(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	// Azure provider may return empty PrivateIP
	mock := &cloudmetadata.MockProvider{
		InstanceID:    "azure-vm-123",
		Region:        "eastus",
		Hostname:      "azure-host",
		PrivateIP:     "", // Empty - should fallback to getIpAddress()
		AccountID:     "azure-subscription",
		CloudProvider: cloudmetadata.CloudProviderAzure,
	}
	cloudmetadata.SetGlobalProviderForTest(mock)

	result := GetMetadataInfo(nil)

	assert.Equal(t, "azure-vm-123", result[instanceIdPlaceholder])
	assert.Equal(t, "eastus", result[awsRegionPlaceholder])
	assert.Equal(t, "azure-host", result[hostnamePlaceholder])
	// Should fallback to local IP detection
	assert.NotEmpty(t, result[ipAddressPlaceholder])
	assert.Equal(t, "azure-subscription", result[accountIdPlaceholder])
}

func TestGetMetadataInfo_SingletonWithEmptyValues(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	// Provider with all empty values
	mock := &cloudmetadata.MockProvider{
		InstanceID: "",
		Region:     "",
		Hostname:   "",
		PrivateIP:  "",
		AccountID:  "",
	}
	cloudmetadata.SetGlobalProviderForTest(mock)

	result := GetMetadataInfo(nil)

	// Should use defaults for empty values
	assert.Equal(t, unknownInstanceID, result[instanceIdPlaceholder])
	assert.Equal(t, unknownAwsRegion, result[awsRegionPlaceholder])
	// Hostname should fallback to local hostname
	assert.Equal(t, getHostName(), result[hostnamePlaceholder])
	// PrivateIP should fallback to local IP
	assert.NotEmpty(t, result[ipAddressPlaceholder])
	assert.Equal(t, unknownAccountID, result[accountIdPlaceholder])
}

// --- Edge Case Tests for Safe Type Assertions ---

func TestResolveAWSMetadataPlaceholders_NonMapInput(t *testing.T) {
	// Test with string input - should return unchanged
	stringInput := "not a map"
	result := ResolveAWSMetadataPlaceholders(stringInput)
	assert.Equal(t, stringInput, result)

	// Test with nil input - should return unchanged
	var nilInput any
	result = ResolveAWSMetadataPlaceholders(nilInput)
	assert.Nil(t, result)

	// Test with slice input - should return unchanged
	sliceInput := []string{"a", "b", "c"}
	result = ResolveAWSMetadataPlaceholders(sliceInput)
	assert.Equal(t, sliceInput, result)

	// Test with int input - should return unchanged
	intInput := 42
	result = ResolveAWSMetadataPlaceholders(intInput)
	assert.Equal(t, intInput, result)
}

func TestResolveAzureMetadataPlaceholders_NonMapInput(t *testing.T) {
	// Test with string input - should return unchanged
	stringInput := "not a map"
	result := ResolveAzureMetadataPlaceholders(stringInput)
	assert.Equal(t, stringInput, result)

	// Test with nil input - should return unchanged
	var nilInput any
	result = ResolveAzureMetadataPlaceholders(nilInput)
	assert.Nil(t, result)

	// Test with slice input - should return unchanged
	sliceInput := []string{"a", "b", "c"}
	result = ResolveAzureMetadataPlaceholders(sliceInput)
	assert.Equal(t, sliceInput, result)
}

func TestResolveCloudMetadataPlaceholders_NonMapInput(t *testing.T) {
	// Test with string input - should return unchanged
	stringInput := "not a map"
	result := ResolveCloudMetadataPlaceholders(stringInput)
	assert.Equal(t, stringInput, result)

	// Test with nil input - should return unchanged
	var nilInput any
	result = ResolveCloudMetadataPlaceholders(nilInput)
	assert.Nil(t, result)

	// Test with int input - should return unchanged
	intInput := 123
	result = ResolveCloudMetadataPlaceholders(intInput)
	assert.Equal(t, intInput, result)
}

func TestGetMetadataInfo_NilProviderWithoutSingleton(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	// No singleton set, nil provider passed - should return defaults
	result := GetMetadataInfo(nil)

	assert.Equal(t, unknownInstanceID, result[instanceIdPlaceholder])
	assert.Equal(t, unknownAwsRegion, result[awsRegionPlaceholder])
	assert.Equal(t, unknownAccountID, result[accountIdPlaceholder])
	// Hostname and IP should be derived from local system
	assert.NotEmpty(t, result[hostnamePlaceholder])
	assert.NotEmpty(t, result[ipAddressPlaceholder])
}

// TestResolveAWSMetadataPlaceholders_EmbeddedPlaceholders tests embedded placeholder support
func TestResolveAWSMetadataPlaceholders_EmbeddedPlaceholders(t *testing.T) {
	// Mock the metadata provider
	tagMetadataProvider = func() map[string]string {
		return map[string]string{}
	}
	defer func() { tagMetadataProvider = nil }()

	ec2MetadataInfoProviderFunc = func() *Metadata {
		return &Metadata{
			InstanceID:   "i-test123",
			InstanceType: "t2.micro",
			ImageID:      "ami-test456",
		}
	}
	defer func() { ec2MetadataInfoProviderFunc = ec2MetadataInfoProvider }()

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "single embedded placeholder",
			input: map[string]interface{}{
				"Name": "prefix-${aws:InstanceId}-suffix",
			},
			expected: map[string]interface{}{
				"Name": "prefix-i-test123-suffix",
			},
		},
		{
			name: "multiple placeholders in one string",
			input: map[string]interface{}{
				"Name": "${aws:InstanceId}-${aws:InstanceType}",
			},
			expected: map[string]interface{}{
				"Name": "i-test123-t2.micro",
			},
		},
		{
			name: "mixed embedded and exact match",
			input: map[string]interface{}{
				"InstanceId": "${aws:InstanceId}",
				"Name":       "server-${aws:InstanceId}",
			},
			expected: map[string]interface{}{
				"InstanceId": "i-test123",
				"Name":       "server-i-test123",
			},
		},
		{
			name: "no placeholders",
			input: map[string]interface{}{
				"Name": "static-value",
			},
			expected: map[string]interface{}{
				"Name": "static-value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveAWSMetadataPlaceholders(tt.input)
			resultMap := result.(map[string]interface{})
			assert.Equal(t, tt.expected, resultMap)
		})
	}
}

// TestResolveAzureMetadataPlaceholders_EmbeddedPlaceholders tests embedded placeholder support for Azure
func TestResolveAzureMetadataPlaceholders_EmbeddedPlaceholders(t *testing.T) {
	// Set up mock Azure provider
	mockProvider := &cloudmetadata.MockProvider{
		InstanceID:    "vm-12345",
		InstanceType:  "Standard_D2s_v3",
		ImageID:       "image-67890",
		CloudProvider: cloudmetadata.CloudProviderAzure,
		ResourceGroup: "my-resource-group",
		Available:     true,
		Tags: map[string]string{
			"VmScaleSetName": "my-vmss",
		},
	}

	cloudmetadata.SetGlobalProviderForTest(mockProvider)
	defer cloudmetadata.ResetGlobalProvider()

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "single embedded placeholder",
			input: map[string]interface{}{
				"Name": "prefix-${azure:InstanceId}-suffix",
			},
			expected: map[string]interface{}{
				"Name": "prefix-vm-12345-suffix",
			},
		},
		{
			name: "multiple placeholders in one string",
			input: map[string]interface{}{
				"Name": "${azure:InstanceId}-${azure:InstanceType}",
			},
			expected: map[string]interface{}{
				"Name": "vm-12345-Standard_D2s_v3",
			},
		},
		{
			name: "resource group embedded",
			input: map[string]interface{}{
				"Path": "/subscriptions/sub/${azure:ResourceGroupName}/vms/${azure:InstanceId}",
			},
			expected: map[string]interface{}{
				"Path": "/subscriptions/sub/my-resource-group/vms/vm-12345",
			},
		},
		{
			name: "mixed embedded and exact match",
			input: map[string]interface{}{
				"InstanceId": "${azure:InstanceId}",
				"Name":       "vm-${azure:InstanceId}",
			},
			expected: map[string]interface{}{
				"InstanceId": "vm-12345",
				"Name":       "vm-vm-12345",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveAzureMetadataPlaceholders(tt.input)
			resultMap := result.(map[string]interface{})
			assert.Equal(t, tt.expected, resultMap)
		})
	}
}

// TestResolveCloudMetadataPlaceholders_MixedEmbedded tests mixed AWS and Azure placeholders
func TestResolveCloudMetadataPlaceholders_MixedEmbedded(t *testing.T) {
	// Mock AWS metadata
	ec2MetadataInfoProviderFunc = func() *Metadata {
		return &Metadata{
			InstanceID: "i-aws123",
		}
	}
	defer func() { ec2MetadataInfoProviderFunc = ec2MetadataInfoProvider }()

	tagMetadataProvider = func() map[string]string {
		return map[string]string{}
	}
	defer func() { tagMetadataProvider = nil }()

	// Set up mock Azure provider
	mockProvider := &cloudmetadata.MockProvider{
		InstanceID:    "vm-azure456",
		CloudProvider: cloudmetadata.CloudProviderAzure,
		Available:     true,
	}

	cloudmetadata.SetGlobalProviderForTest(mockProvider)
	defer cloudmetadata.ResetGlobalProvider()

	input := map[string]interface{}{
		"AWSName":   "aws-${aws:InstanceId}",
		"AzureName": "azure-${azure:InstanceId}",
		"Mixed":     "${aws:InstanceId}-and-${azure:InstanceId}",
	}

	result := ResolveCloudMetadataPlaceholders(input)
	resultMap := result.(map[string]interface{})

	assert.Equal(t, "aws-i-aws123", resultMap["AWSName"])
	assert.Equal(t, "azure-vm-azure456", resultMap["AzureName"])
	assert.Equal(t, "i-aws123-and-vm-azure456", resultMap["Mixed"])
}
