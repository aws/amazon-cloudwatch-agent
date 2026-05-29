// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tagutil

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEC2TagsClient is a mock implementation for testing
type MockEC2TagsClient struct {
	mock.Mock
}

func (m *MockEC2TagsClient) DescribeTags(ctx context.Context, input *ec2.DescribeTagsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(*ec2.DescribeTagsOutput), args.Error(1)
}

func TestGetAutoScalingGroupName(t *testing.T) {
	ResetTagsCache()

	mockClient := &MockEC2TagsClient{}
	mockTags := []types.TagDescription{
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

	mockClient.On("DescribeTags", mock.Anything, mock.Anything).Return(mockOutput, nil)

	SetEC2APIProviderForTesting(func() ec2.DescribeTagsAPIClient {
		return mockClient
	})

	result := GetAutoScalingGroupName(t.Context(), "i-1234567890abcdef0")
	assert.Equal(t, "my-asg-group", result)
	mockClient.AssertExpectations(t)

	// Clean up
	ResetEC2APIProvider()
	ResetTagsCache()
}

func TestGetEKSClusterName(t *testing.T) {
	ResetTagsCache()

	mockClient := &MockEC2TagsClient{}
	mockTags := []types.TagDescription{
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

	mockClient.On("DescribeTags", mock.Anything, mock.Anything).Return(mockOutput, nil)

	SetEC2APIProviderForTesting(func() ec2.DescribeTagsAPIClient {
		return mockClient
	})

	result := GetEKSClusterName(t.Context(), "i-1234567890abcdef0")
	assert.Equal(t, "my-eks-cluster", result)
	mockClient.AssertExpectations(t)

	// Clean up
	ResetEC2APIProvider()
	ResetTagsCache()
}

func TestGetEKSClusterName_EmptyResult(t *testing.T) {
	ResetTagsCache()

	mockClient := &MockEC2TagsClient{}
	mockOutput := &ec2.DescribeTagsOutput{
		Tags: []types.TagDescription{},
	}

	mockClient.On("DescribeTags", mock.Anything, mock.Anything).Return(mockOutput, nil)
	SetEC2APIProviderForTesting(func() ec2.DescribeTagsAPIClient {
		return mockClient
	})

	result := GetEKSClusterName(t.Context(), "i-1234567890abcdef0")
	assert.Equal(t, "", result)

	// Clean up
	ResetEC2APIProvider()
	ResetTagsCache()
}
func TestLoadAllTagsWithPagination(t *testing.T) {
	// Reset cache before test
	ResetTagsCache()

	instanceID := "i-1234567890abcdef0"

	// Create a custom mock client that handles pagination
	paginatedClient := &MockEC2TagsClientWithPagination{}

	// Set the mock provider with our custom client
	SetEC2APIProviderForTesting(func() ec2.DescribeTagsAPIClient {
		return paginatedClient
	})

	defer func() {
		ResetEC2APIProvider()
		ResetTagsCache()
	}()

	// Test GetEKSClusterName to trigger loadAllTags and verify all tags were loaded
	clusterName := GetEKSClusterName(t.Context(), instanceID)
	assert.Equal(t, "my-cluster", clusterName)

	// Verify that both API calls were made (pagination worked)
	assert.Equal(t, 2, paginatedClient.callCount)
}

// MockEC2TagsClientWithPagination is a custom mock that handles pagination
type MockEC2TagsClientWithPagination struct {
	callCount int
}

func (m *MockEC2TagsClientWithPagination) DescribeTags(context.Context, *ec2.DescribeTagsInput, ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
	m.callCount++

	if m.callCount == 1 {
		// First page of results
		return &ec2.DescribeTagsOutput{
			Tags: []types.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("test-instance"),
				},
				{
					Key:   aws.String("Environment"),
					Value: aws.String("production"),
				},
			},
			NextToken: aws.String("next-page-token"),
		}, nil
	}
	// Second page of results
	return &ec2.DescribeTagsOutput{
		Tags: []types.TagDescription{
			{
				Key:   aws.String("kubernetes.io/cluster/my-cluster"),
				Value: aws.String("owned"),
			},
			{
				Key:   aws.String("Team"),
				Value: aws.String("backend"),
			},
		},
		NextToken: nil, // No more pages
	}, nil
}
