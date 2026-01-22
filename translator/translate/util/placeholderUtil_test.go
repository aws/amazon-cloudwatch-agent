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
		InstanceID_: "i-singleton123",
		Region_:     "us-west-2",
		Hostname_:   "singleton-host",
		PrivateIP_:  "192.168.1.1",
		AccountID_:  "999888777666",
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
		InstanceID_: "i-singleton",
		Region_:     "singleton-region",
		Hostname_:   "singleton-host",
		PrivateIP_:  "10.1.1.1",
		AccountID_:  "singleton-account",
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
		InstanceID_:    "azure-vm-123",
		Region_:        "eastus",
		Hostname_:      "azure-host",
		PrivateIP_:     "", // Empty - should fallback to getIpAddress()
		AccountID_:     "azure-subscription",
		CloudProvider_: cloudmetadata.CloudProviderAzure,
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
		InstanceID_: "",
		Region_:     "",
		Hostname_:   "",
		PrivateIP_:  "",
		AccountID_:  "",
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

// --- Tests for AWS Placeholders with Cloudmetadata Provider ---

func TestResolveAWSMetadataPlaceholders_WithCloudmetadataProvider(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	// Set up AWS cloudmetadata provider
	mock := &cloudmetadata.MockProvider{
		InstanceID_:    "i-cloudmeta123",
		InstanceType_:  "m5.xlarge",
		ImageID_:       "ami-cloudmeta456",
		Region_:        "us-east-1",
		AccountID_:     "123456789012",
		AZ_:            "us-east-1a",
		Hostname_:      "cloudmeta-host",
		PrivateIP_:     "10.0.0.50",
		CloudProvider_: cloudmetadata.CloudProviderAWS,
	}
	cloudmetadata.SetGlobalProviderForTest(mock)

	input := map[string]any{
		"InstanceId":   "${aws:InstanceId}",
		"InstanceType": "${aws:InstanceType}",
		"ImageId":      "${aws:ImageId}",
		"RegularKey":   "regular_value",
	}

	result := ResolveAWSMetadataPlaceholders(input)
	resultMap := result.(map[string]any)

	assert.Equal(t, "i-cloudmeta123", resultMap["InstanceId"])
	assert.Equal(t, "m5.xlarge", resultMap["InstanceType"])
	assert.Equal(t, "ami-cloudmeta456", resultMap["ImageId"])
	assert.Equal(t, "regular_value", resultMap["RegularKey"])
}

func TestResolveAWSMetadataPlaceholders_ExtendedPlaceholders(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	mock := &cloudmetadata.MockProvider{
		InstanceID_:    "i-extended123",
		InstanceType_:  "t3.medium",
		ImageID_:       "ami-extended456",
		Region_:        "eu-west-1",
		AccountID_:     "987654321098",
		AZ_:            "eu-west-1b",
		Hostname_:      "extended-host",
		PrivateIP_:     "172.16.0.100",
		CloudProvider_: cloudmetadata.CloudProviderAWS,
	}
	cloudmetadata.SetGlobalProviderForTest(mock)

	input := map[string]any{
		"Region":           "${aws:Region}",
		"AccountId":        "${aws:AccountId}",
		"AvailabilityZone": "${aws:AvailabilityZone}",
		"Hostname":         "${aws:Hostname}",
		"PrivateIP":        "${aws:PrivateIP}",
	}

	result := ResolveAWSMetadataPlaceholders(input)
	resultMap := result.(map[string]any)

	assert.Equal(t, "eu-west-1", resultMap["Region"])
	assert.Equal(t, "987654321098", resultMap["AccountId"])
	assert.Equal(t, "eu-west-1b", resultMap["AvailabilityZone"])
	assert.Equal(t, "extended-host", resultMap["Hostname"])
	assert.Equal(t, "172.16.0.100", resultMap["PrivateIP"])
}

// --- Tests for Azure Placeholders with Cloudmetadata Provider ---

func TestResolveAzureMetadataPlaceholders_WithCloudmetadataProvider(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	mock := &cloudmetadata.MockProvider{
		InstanceID_:       "azure-vm-12345",
		InstanceType_:     "Standard_D4s_v3",
		Region_:           "eastus2",
		AccountID_:        "sub-12345-67890",
		AZ_:               "1",
		Hostname_:         "azure-vm-host",
		PrivateIP_:        "10.1.0.50",
		ScalingGroupName_: "my-vmss",
		CloudProvider_:    cloudmetadata.CloudProviderAzure,
	}
	cloudmetadata.SetGlobalProviderForTest(mock)

	input := map[string]any{
		"InstanceId":     "${azure:InstanceId}",
		"InstanceType":   "${azure:InstanceType}",
		"VmScaleSetName": "${azure:VmScaleSetName}",
		"Region":         "${azure:Region}",
		"SubscriptionId": "${azure:SubscriptionId}",
		"RegularKey":     "regular_value",
	}

	result := ResolveAzureMetadataPlaceholders(input)
	resultMap := result.(map[string]any)

	assert.Equal(t, "azure-vm-12345", resultMap["InstanceId"])
	assert.Equal(t, "Standard_D4s_v3", resultMap["InstanceType"])
	assert.Equal(t, "my-vmss", resultMap["VmScaleSetName"])
	assert.Equal(t, "eastus2", resultMap["Region"])
	assert.Equal(t, "sub-12345-67890", resultMap["SubscriptionId"])
	assert.Equal(t, "regular_value", resultMap["RegularKey"])
}

func TestResolveAzureMetadataPlaceholders_ExtendedPlaceholders(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	mock := &cloudmetadata.MockProvider{
		InstanceID_:    "azure-extended-vm",
		InstanceType_:  "Standard_E8s_v4",
		Region_:        "westeurope",
		AccountID_:     "sub-extended-12345",
		AZ_:            "2",
		Hostname_:      "azure-extended-host",
		PrivateIP_:     "10.2.0.100",
		CloudProvider_: cloudmetadata.CloudProviderAzure,
	}
	cloudmetadata.SetGlobalProviderForTest(mock)

	input := map[string]any{
		"AvailabilityZone": "${azure:AvailabilityZone}",
		"Hostname":         "${azure:Hostname}",
		"PrivateIP":        "${azure:PrivateIP}",
	}

	result := ResolveAzureMetadataPlaceholders(input)
	resultMap := result.(map[string]any)

	assert.Equal(t, "2", resultMap["AvailabilityZone"])
	assert.Equal(t, "azure-extended-host", resultMap["Hostname"])
	assert.Equal(t, "10.2.0.100", resultMap["PrivateIP"])
}

// --- Tests for Cloud-Agnostic Placeholders ---

func TestResolveCloudAgnosticPlaceholders_AWS(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	mock := &cloudmetadata.MockProvider{
		InstanceID_:    "i-aws-agnostic",
		InstanceType_:  "c5.2xlarge",
		Region_:        "ap-southeast-1",
		AccountID_:     "111222333444",
		AZ_:            "ap-southeast-1a",
		Hostname_:      "aws-agnostic-host",
		PrivateIP_:     "10.10.10.10",
		CloudProvider_: cloudmetadata.CloudProviderAWS,
	}
	cloudmetadata.SetGlobalProviderForTest(mock)

	input := map[string]any{
		"InstanceId":       "${cloud:InstanceId}",
		"InstanceType":     "${cloud:InstanceType}",
		"Region":           "${cloud:Region}",
		"AccountId":        "${cloud:AccountId}",
		"AvailabilityZone": "${cloud:AvailabilityZone}",
		"Hostname":         "${cloud:Hostname}",
		"PrivateIP":        "${cloud:PrivateIP}",
	}

	result := ResolveCloudAgnosticPlaceholders(input)
	resultMap := result.(map[string]any)

	assert.Equal(t, "i-aws-agnostic", resultMap["InstanceId"])
	assert.Equal(t, "c5.2xlarge", resultMap["InstanceType"])
	assert.Equal(t, "ap-southeast-1", resultMap["Region"])
	assert.Equal(t, "111222333444", resultMap["AccountId"])
	assert.Equal(t, "ap-southeast-1a", resultMap["AvailabilityZone"])
	assert.Equal(t, "aws-agnostic-host", resultMap["Hostname"])
	assert.Equal(t, "10.10.10.10", resultMap["PrivateIP"])
}

func TestResolveCloudAgnosticPlaceholders_Azure(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	mock := &cloudmetadata.MockProvider{
		InstanceID_:    "azure-agnostic-vm",
		InstanceType_:  "Standard_D8s_v3",
		Region_:        "northeurope",
		AccountID_:     "azure-sub-12345",
		AZ_:            "3",
		Hostname_:      "azure-agnostic-host",
		PrivateIP_:     "10.20.30.40",
		CloudProvider_: cloudmetadata.CloudProviderAzure,
	}
	cloudmetadata.SetGlobalProviderForTest(mock)

	input := map[string]any{
		"InstanceId":   "${cloud:InstanceId}",
		"InstanceType": "${cloud:InstanceType}",
		"Region":       "${cloud:Region}",
		"AccountId":    "${cloud:AccountId}",
	}

	result := ResolveCloudAgnosticPlaceholders(input)
	resultMap := result.(map[string]any)

	assert.Equal(t, "azure-agnostic-vm", resultMap["InstanceId"])
	assert.Equal(t, "Standard_D8s_v3", resultMap["InstanceType"])
	assert.Equal(t, "northeurope", resultMap["Region"])
	assert.Equal(t, "azure-sub-12345", resultMap["AccountId"])
}

func TestResolveCloudAgnosticPlaceholders_NoProvider(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	// Don't set provider - test fallback behavior

	input := map[string]any{
		"InstanceId": "${cloud:InstanceId}",
		"Region":     "${cloud:Region}",
	}

	result := ResolveCloudAgnosticPlaceholders(input)
	resultMap := result.(map[string]any)

	// Should use defaults when no provider available
	assert.Equal(t, unknownInstanceID, resultMap["InstanceId"])
	assert.Equal(t, unknownAwsRegion, resultMap["Region"])
}

// --- Tests for ResolveCloudMetadataPlaceholders (unified resolver) ---

func TestResolveCloudMetadataPlaceholders_MixedPlaceholders(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	mock := &cloudmetadata.MockProvider{
		InstanceID_:    "i-mixed123",
		InstanceType_:  "t3.large",
		Region_:        "us-west-2",
		AccountID_:     "555666777888",
		CloudProvider_: cloudmetadata.CloudProviderAWS,
	}
	cloudmetadata.SetGlobalProviderForTest(mock)

	input := map[string]any{
		"AWSInstanceId": "${aws:InstanceId}",
		"CloudRegion":   "${cloud:Region}",
		"RegularKey":    "regular_value",
	}

	result := ResolveCloudMetadataPlaceholders(input)
	resultMap := result.(map[string]any)

	assert.Equal(t, "i-mixed123", resultMap["AWSInstanceId"])
	assert.Equal(t, "us-west-2", resultMap["CloudRegion"])
	assert.Equal(t, "regular_value", resultMap["RegularKey"])
}

func TestResolveCloudMetadataPlaceholders_NoPlaceholders(t *testing.T) {
	input := map[string]any{
		"key1": "value1",
		"key2": "value2",
	}

	result := ResolveCloudMetadataPlaceholders(input)
	resultMap := result.(map[string]any)

	assert.Equal(t, "value1", resultMap["key1"])
	assert.Equal(t, "value2", resultMap["key2"])
}

// --- Tests for fallback behavior ---

func TestGetAWSMetadata_FallbackToLegacy(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	// Don't set provider - should fallback to legacy

	originalProvider := Ec2MetadataInfoProvider
	Ec2MetadataInfoProvider = func() *Metadata {
		return &Metadata{
			InstanceID:   "i-legacy-fallback",
			InstanceType: "t2.micro",
			ImageID:      "ami-legacy-fallback",
		}
	}
	defer func() {
		Ec2MetadataInfoProvider = originalProvider
	}()

	metadata := getAWSMetadata()

	assert.Equal(t, "i-legacy-fallback", metadata["${aws:InstanceId}"])
	assert.Equal(t, "t2.micro", metadata["${aws:InstanceType}"])
	assert.Equal(t, "ami-legacy-fallback", metadata["${aws:ImageId}"])
}

func TestGetAWSMetadata_UsesCloudmetadataWhenAvailable(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	mock := &cloudmetadata.MockProvider{
		InstanceID_:    "i-cloudmeta-preferred",
		InstanceType_:  "m5.large",
		ImageID_:       "ami-cloudmeta-preferred",
		Region_:        "eu-central-1",
		CloudProvider_: cloudmetadata.CloudProviderAWS,
	}
	cloudmetadata.SetGlobalProviderForTest(mock)

	// Also set legacy (should be ignored)
	originalProvider := Ec2MetadataInfoProvider
	Ec2MetadataInfoProvider = func() *Metadata {
		return &Metadata{
			InstanceID:   "i-legacy-ignored",
			InstanceType: "t2.nano",
			ImageID:      "ami-legacy-ignored",
		}
	}
	defer func() {
		Ec2MetadataInfoProvider = originalProvider
	}()

	metadata := getAWSMetadata()

	// Cloudmetadata should win
	assert.Equal(t, "i-cloudmeta-preferred", metadata["${aws:InstanceId}"])
	assert.Equal(t, "m5.large", metadata["${aws:InstanceType}"])
	assert.Equal(t, "ami-cloudmeta-preferred", metadata["${aws:ImageId}"])
}

func TestGetAzureMetadata_UsesCloudmetadataWhenAvailable(t *testing.T) {
	cloudmetadata.ResetGlobalProvider()
	defer cloudmetadata.ResetGlobalProvider()

	mock := &cloudmetadata.MockProvider{
		InstanceID_:       "azure-cloudmeta-vm",
		InstanceType_:     "Standard_D2s_v3",
		Region_:           "australiaeast",
		AccountID_:        "azure-sub-cloudmeta",
		ScalingGroupName_: "cloudmeta-vmss",
		CloudProvider_:    cloudmetadata.CloudProviderAzure,
	}
	cloudmetadata.SetGlobalProviderForTest(mock)

	metadata := getAzureMetadata()

	assert.Equal(t, "azure-cloudmeta-vm", metadata["${azure:InstanceId}"])
	assert.Equal(t, "Standard_D2s_v3", metadata["${azure:InstanceType}"])
	assert.Equal(t, "australiaeast", metadata["${azure:Region}"])
	assert.Equal(t, "azure-sub-cloudmeta", metadata["${azure:SubscriptionId}"])
	assert.Equal(t, "cloudmeta-vmss", metadata["${azure:VmScaleSetName}"])
}
