// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
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
	ResetAWSMetadataCache()

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
		ResetAWSMetadataCache()
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
func TestAWSMetadataCaching(t *testing.T) {

	callCount := 0
	originalProvider := Ec2MetadataInfoProvider
	Ec2MetadataInfoProvider = func() *Metadata {
		callCount++
		return &Metadata{
			InstanceID:   "i-test123",
			InstanceType: "t3.micro",
			ImageID:      "ami-test123",
		}
	}
	defer func() {
		Ec2MetadataInfoProvider = originalProvider
	}()

	// First call should fetch metadata
	input1 := map[string]interface{}{
		"InstanceId": "${aws:InstanceId}",
	}
	result1 := ResolveAWSMetadataPlaceholders(input1)
	assert.Equal(t, 1, callCount, "Metadata provider should be called once")

	resultMap1 := result1.(map[string]interface{})
	assert.Equal(t, "i-test123", resultMap1["InstanceId"])

	// Second call should use cached metadata
	input2 := map[string]interface{}{
		"InstanceType": "${aws:InstanceType}",
	}
	result2 := ResolveAWSMetadataPlaceholders(input2)
	assert.Equal(t, 1, callCount, "Metadata provider should still be called only once (cached)")

	resultMap2 := result2.(map[string]interface{})
	assert.Equal(t, "t3.micro", resultMap2["InstanceType"])

	// Third call with multiple placeholders should still use cache
	input3 := map[string]interface{}{
		"InstanceId":   "${aws:InstanceId}",
		"InstanceType": "${aws:InstanceType}",
		"ImageId":      "${aws:ImageId}",
	}
	result3 := ResolveAWSMetadataPlaceholders(input3)
	assert.Equal(t, 1, callCount, "Metadata provider should still be called only once (cached)")

	resultMap3 := result3.(map[string]interface{})
	assert.Equal(t, "i-test123", resultMap3["InstanceId"])
	assert.Equal(t, "t3.micro", resultMap3["InstanceType"])
	assert.Equal(t, "ami-test123", resultMap3["ImageId"])
}
