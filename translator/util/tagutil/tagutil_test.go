// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tagutil

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEC2TagsClient is a mock implementation of EC2TagsClient for testing
type MockEC2TagsClient struct {
	mock.Mock
}

func (m *MockEC2TagsClient) DescribeTagsWithContext(ctx aws.Context, input *ec2.DescribeTagsInput, opts ...request.Option) (*ec2.DescribeTagsOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*ec2.DescribeTagsOutput), args.Error(1)
}

func TestGetAutoScalingGroupName(t *testing.T) {
	// Reset the cache before test
	ResetTagsCache()

	// Create mock client
	mockClient := &MockEC2TagsClient{}

	// Set up mock response with ASG tag
	mockTags := []*ec2.TagDescription{
		{
			Key:   aws.String("aws:autoscaling:groupName"),
			Value: aws.String("my-asg-group"),
		},
		{
			Key:   aws.String("Name"),
			Value: aws.String("test-instance"),
		},
	}

	mockOutput := &ec2.DescribeTagsOutput{
		Tags: mockTags,
	}

	mockClient.On("DescribeTagsWithContext", mock.Anything, mock.Anything, mock.Anything).Return(mockOutput, nil)

	// Set the mock provider
	SetEC2APIProviderForTesting(func() EC2TagsClient {
		return mockClient
	})

	// Test the function
	result := GetAutoScalingGroupName("i-1234567890abcdef0")

	// Verify results
	assert.Equal(t, "my-asg-group", result)

	// Verify mock was called
	mockClient.AssertExpectations(t)

	// Clean up
	ResetEC2APIProvider()
	ResetTagsCache()
}

func TestGetEKSClusterName(t *testing.T) {
	// Reset the cache before test
	ResetTagsCache()

	// Create mock client
	mockClient := &MockEC2TagsClient{}

	// Set up mock response with EKS cluster tag
	mockTags := []*ec2.TagDescription{
		{
			Key:   aws.String("kubernetes.io/cluster/my-eks-cluster"),
			Value: aws.String("owned"),
		},
		{
			Key:   aws.String("Name"),
			Value: aws.String("test-instance"),
		},
	}

	mockOutput := &ec2.DescribeTagsOutput{
		Tags: mockTags,
	}

	mockClient.On("DescribeTagsWithContext", mock.Anything, mock.Anything, mock.Anything).Return(mockOutput, nil)

	// Set the mock provider
	SetEC2APIProviderForTesting(func() EC2TagsClient {
		return mockClient
	})

	// Test the function
	result := GetEKSClusterName("i-1234567890abcdef0")

	// Verify results
	assert.Equal(t, "my-eks-cluster", result)

	// Verify mock was called
	mockClient.AssertExpectations(t)

	// Clean up
	ResetEC2APIProvider()
	ResetTagsCache()
}

func TestGetEKSClusterName_EmptyResult(t *testing.T) {
	// Reset the cache before test
	ResetTagsCache()

	// Create mock client
	mockClient := &MockEC2TagsClient{}

	// Set up empty mock response
	mockOutput := &ec2.DescribeTagsOutput{
		Tags: []*ec2.TagDescription{},
	}

	mockClient.On("DescribeTagsWithContext", mock.Anything, mock.Anything, mock.Anything).Return(mockOutput, nil)

	// Set the mock provider
	SetEC2APIProviderForTesting(func() EC2TagsClient {
		return mockClient
	})

	// Test the function
	result := GetEKSClusterName("i-1234567890abcdef0")

	// Verify results
	assert.Equal(t, "", result)

	// Clean up
	ResetEC2APIProvider()
	ResetTagsCache()
}
