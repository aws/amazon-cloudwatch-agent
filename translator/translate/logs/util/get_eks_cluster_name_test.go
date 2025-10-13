// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/translator/util/tagutil"
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

func TestGetEKSClusterName(t *testing.T) {
	tests := []struct {
		name           string
		instanceID     string
		mockTags       []*ec2.TagDescription
		expectedResult string
	}{
		{
			name:       "EKS cluster tag found",
			instanceID: "i-1234567890abcdef0",
			mockTags: []*ec2.TagDescription{
				{
					Key:   aws.String("kubernetes.io/cluster/my-eks-cluster"),
					Value: aws.String("owned"),
				},
				{
					Key:   aws.String("Name"),
					Value: aws.String("test-instance"),
				},
			},
			expectedResult: "my-eks-cluster",
		},
		{
			name:       "Multiple EKS cluster tags, returns first found",
			instanceID: "i-1234567890abcdef0",
			mockTags: []*ec2.TagDescription{
				{
					Key:   aws.String("kubernetes.io/cluster/cluster-a"),
					Value: aws.String("owned"),
				},
				{
					Key:   aws.String("kubernetes.io/cluster/cluster-b"),
					Value: aws.String("owned"),
				},
				{
					Key:   aws.String("Name"),
					Value: aws.String("test-instance"),
				},
			},
			expectedResult: "cluster-a", // Should return one of them
		},
		{
			name:       "EKS cluster tag with wrong value",
			instanceID: "i-1234567890abcdef0",
			mockTags: []*ec2.TagDescription{
				{
					Key:   aws.String("kubernetes.io/cluster/my-cluster"),
					Value: aws.String("shared"), // Not "owned"
				},
				{
					Key:   aws.String("Name"),
					Value: aws.String("test-instance"),
				},
			},
			expectedResult: "",
		},
		{
			name:       "No EKS cluster tags",
			instanceID: "i-1234567890abcdef0",
			mockTags: []*ec2.TagDescription{
				{
					Key:   aws.String("Name"),
					Value: aws.String("test-instance"),
				},
				{
					Key:   aws.String("Environment"),
					Value: aws.String("production"),
				},
			},
			expectedResult: "",
		},
		{
			name:           "No tags at all",
			instanceID:     "i-1234567890abcdef0",
			mockTags:       []*ec2.TagDescription{},
			expectedResult: "",
		},
		{
			name:           "Empty instance ID",
			instanceID:     "",
			mockTags:       nil,
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset cache before test
			tagutil.ResetTagsCache()

			if tt.mockTags != nil {
				// Create mock client
				mockClient := &MockEC2TagsClient{}
				mockOutput := &ec2.DescribeTagsOutput{
					Tags: tt.mockTags,
				}
				mockClient.On("DescribeTagsWithContext", mock.Anything, mock.Anything, mock.Anything).Return(mockOutput, nil)

				tagutil.SetEC2APIProviderForTesting(func() tagutil.EC2TagsClient {
					return mockClient
				})

				defer func() {
					tagutil.ResetEC2APIProvider()
					tagutil.ResetTagsCache()
				}()
			}

			result := tagutil.GetEKSClusterName(tt.instanceID)

			if tt.name == "Multiple EKS cluster tags, returns first found" {
				assert.True(t, result == "cluster-a" || result == "cluster-b",
					"Expected cluster-a or cluster-b, got %s", result)
			} else {
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
