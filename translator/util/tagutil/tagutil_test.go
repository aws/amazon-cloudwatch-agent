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

func TestGetAllTagsForInstance(t *testing.T) {
	// Reset the cache before test
	ResetTagsCache()

	// Create mock client
	mockClient := &MockEC2TagsClient{}

	// Set up mock response
	mockTags := []*ec2.TagDescription{
		{
			Key:   aws.String("Name"),
			Value: aws.String("test-instance"),
		},
		{
			Key:   aws.String("Environment"),
			Value: aws.String("test"),
		},
		{
			Key:   aws.String("kubernetes.io/cluster/my-cluster"),
			Value: aws.String("owned"),
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
	result := GetAllTagsForInstance("i-1234567890abcdef0")

	// Verify results
	expected := map[string]string{
		"Name":                             "test-instance",
		"Environment":                      "test",
		"kubernetes.io/cluster/my-cluster": "owned",
	}

	assert.Equal(t, expected, result)

	// Verify mock was called
	mockClient.AssertExpectations(t)

	// Clean up
	ResetEC2APIProvider()
	ResetTagsCache()
}

func TestGetAllTagsForInstance_EmptyResponse(t *testing.T) {
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
	result := GetAllTagsForInstance("i-1234567890abcdef0")

	// Verify results
	assert.Empty(t, result)

	// Verify mock was called
	mockClient.AssertExpectations(t)

	// Clean up
	ResetEC2APIProvider()
	ResetTagsCache()
}
func TestGetAllTagsForInstanceWithRetries(t *testing.T) {
	// Reset the cache before test
	ResetTagsCache()

	// Create mock client
	mockClient := &MockEC2TagsClient{}

	// Set up mock response with tags
	mockTags := []*ec2.TagDescription{
		{
			Key:   aws.String("kubernetes.io/cluster/my-cluster"),
			Value: aws.String("owned"),
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
	result := GetAllTagsForInstanceWithRetries("i-1234567890abcdef0")

	// Verify results
	expected := map[string]string{
		"kubernetes.io/cluster/my-cluster": "owned",
	}

	assert.Equal(t, expected, result)

	// Verify mock was called
	mockClient.AssertExpectations(t)

	// Clean up
	ResetEC2APIProvider()
	ResetTagsCache()
}
