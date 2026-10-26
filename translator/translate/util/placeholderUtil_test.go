// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"

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
	assert.Equal(t, UnknownInstanceID, m[instanceIdPlaceholder])
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

// mockAutoScalingGroupName overrides the ASG name lookup and returns a pointer
// to a call counter plus a cleanup function.
func mockAutoScalingGroupName(name string) (*int, func()) {
	calls := 0
	original := autoScalingGroupNameProvider
	autoScalingGroupNameProvider = func(string) string {
		calls++
		return name
	}
	return &calls, func() { autoScalingGroupNameProvider = original }
}

func TestResolvePlaceholderAutoScalingGroupName(t *testing.T) {
	metadata := map[string]string{instanceIdPlaceholder: "i-1234567890abcdef0"}

	tests := []struct {
		name     string
		asgName  string
		input    string
		expected string
	}{
		{
			name:     "ASG present",
			asgName:  "my-asg",
			input:    "/app/logs/{autoscaling_group_name}",
			expected: "/app/logs/my-asg",
		},
		{
			name:     "ASG present combined with other placeholders",
			asgName:  "my-asg",
			input:    "{autoscaling_group_name}/{instance_id}",
			expected: "my-asg/i-1234567890abcdef0",
		},
		{
			name:     "ASG name with invalid characters is sanitized",
			asgName:  "my asg:prod*1",
			input:    "/app/{autoscaling_group_name}",
			expected: "/app/my_asg_prod_1",
		},
		{
			name:     "ASG absent uses unknown fallback",
			input:    "/app/logs/{autoscaling_group_name}",
			expected: "/app/logs/" + unknownASGName,
		},
		{
			name:     "ASG absent combined with other placeholders",
			input:    "{autoscaling_group_name}/{instance_id}",
			expected: unknownASGName + "/i-1234567890abcdef0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls, cleanup := mockAutoScalingGroupName(tt.asgName)
			defer cleanup()
			assert.Equal(t, tt.expected, ResolvePlaceholder(tt.input, metadata))
			assert.Equal(t, 1, *calls)
		})
	}
}

func TestResolvePlaceholderAutoScalingGroupNameNotUsed(t *testing.T) {
	calls, cleanup := mockAutoScalingGroupName("my-asg")
	defer cleanup()

	metadata := map[string]string{instanceIdPlaceholder: "i-1234567890abcdef0"}
	assert.Equal(t, "/app/i-1234567890abcdef0", ResolvePlaceholder("/app/{instance_id}", metadata))
	assert.Equal(t, 0, *calls, "ASG lookup must not happen when the placeholder is not used")
}

func TestResolvePlaceholderAutoScalingGroupNameUnknownInstance(t *testing.T) {
	calls, cleanup := mockAutoScalingGroupName("my-asg")
	defer cleanup()

	for _, id := range []string{"", UnknownInstanceID} {
		metadata := map[string]string{instanceIdPlaceholder: id}
		assert.Equal(t, "/app/"+unknownASGName, ResolvePlaceholder("/app/{autoscaling_group_name}", metadata))
	}
	assert.Equal(t, 0, *calls, "ASG lookup must not happen without a known instance ID")
}

func TestResolvePlaceholderAutoScalingGroupNameFromTags(t *testing.T) {
	tagutil.ResetTagsCache()
	mockClient := &mockDescribeTagsClient{
		output: &ec2.DescribeTagsOutput{
			Tags: []types.TagDescription{
				{Key: aws.String("aws:autoscaling:groupName"), Value: aws.String("tagged-asg")},
			},
		},
	}
	tagutil.SetEC2APIProviderForTesting(func() ec2.DescribeTagsAPIClient { return mockClient })
	defer func() {
		tagutil.ResetEC2APIProvider()
		tagutil.ResetTagsCache()
	}()

	metadata := map[string]string{instanceIdPlaceholder: "i-1234567890abcdef0"}
	assert.Equal(t, "/app/tagged-asg", ResolvePlaceholder("/app/{autoscaling_group_name}", metadata))
	assert.Equal(t, "tagged-asg", ResolvePlaceholder("{autoscaling_group_name}", metadata))
	assert.Equal(t, 1, mockClient.calls, "tags should be fetched once and cached")
}

type mockDescribeTagsClient struct {
	output *ec2.DescribeTagsOutput
	calls  int
}

func (m *mockDescribeTagsClient) DescribeTags(context.Context, *ec2.DescribeTagsInput, ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
	m.calls++
	return m.output, nil
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

// mockEC2Metadata sets up a mock EC2 metadata provider and returns a cleanup
// function that restores the original.
func mockEC2Metadata(m *Metadata) func() {
	original := Ec2MetadataInfoProvider
	Ec2MetadataInfoProvider = func() *Metadata { return m }
	return func() { Ec2MetadataInfoProvider = original }
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

	cleanup := mockEC2Metadata(&Metadata{
		InstanceID:   "i-1234567890abcdef0",
		InstanceType: "t3.large",
		ImageID:      "ami-0abcdef1234567890",
		Hostname:     "test-hostname",
		PrivateIP:    "10.0.1.100",
		AccountID:    "123456789012",
	})

	// Mock the tag metadata provider for this test
	originalTagProvider := tagMetadataProvider
	tagMetadataProvider = func() map[string]string {
		return map[string]string{
			ec2tagger.SupportedAppendDimensions["AutoScalingGroupName"]: "my-test-asg",
		}
	}

	defer func() {
		cleanup()
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

	cleanup := mockEC2Metadata(&Metadata{
		InstanceID:   "i-test123",
		InstanceType: "t3.micro",
		ImageID:      "ami-test123",
	})
	defer cleanup()

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
