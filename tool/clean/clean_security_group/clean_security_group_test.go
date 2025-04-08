// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
)

// Mock EC2 client for testing
type mockEC2Client struct {
	describeSecurityGroupsOutput    *ec2.DescribeSecurityGroupsOutput
	describeNetworkInterfacesOutput *ec2.DescribeNetworkInterfacesOutput
	deleteSecurityGroupCalled       bool
	deleteSecurityGroupInput        *ec2.DeleteSecurityGroupInput
	revokeIngressCalled             bool
	revokeEgressCalled              bool
}

func (m *mockEC2Client) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	if params != nil && len(params.GroupIds) > 0 {
		// When called with specific GroupIds, return only those groups
		matchingGroups := []types.SecurityGroup{}
		for _, sg := range m.describeSecurityGroupsOutput.SecurityGroups {
			for _, requestedID := range params.GroupIds {
				if *sg.GroupId == requestedID {
					matchingGroups = append(matchingGroups, sg)
					break
				}
			}
		}
		return &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: matchingGroups,
		}, nil
	}
	return m.describeSecurityGroupsOutput, nil
}

func (m *mockEC2Client) DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return m.describeNetworkInterfacesOutput, nil
}

func (m *mockEC2Client) DeleteSecurityGroup(ctx context.Context, params *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
	m.deleteSecurityGroupCalled = true
	m.deleteSecurityGroupInput = params
	return &ec2.DeleteSecurityGroupOutput{}, nil
}

func (m *mockEC2Client) RevokeSecurityGroupIngress(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error) {
	m.revokeIngressCalled = true
	return &ec2.RevokeSecurityGroupIngressOutput{}, nil
}

func (m *mockEC2Client) RevokeSecurityGroupEgress(ctx context.Context, params *ec2.RevokeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupEgressOutput, error) {
	m.revokeEgressCalled = true
	return &ec2.RevokeSecurityGroupEgressOutput{}, nil
}

func TestIsDefaultSecurityGroup(t *testing.T) {
	tests := []struct {
		name          string
		securityGroup types.SecurityGroup
		expected      bool
	}{
		{
			name: "Default security group",
			securityGroup: types.SecurityGroup{
				GroupId:   aws.String("sg-12345"),
				GroupName: aws.String("default"),
			},
			expected: true,
		},
		{
			name: "Non-default security group",
			securityGroup: types.SecurityGroup{
				GroupId:   aws.String("sg-67890"),
				GroupName: aws.String("my-security-group"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDefaultSecurityGroup(tt.securityGroup)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSecurityGroupException(t *testing.T) {
	// Set up test exception list
	cfg.exceptionList = []string{"default", "special"}

	tests := []struct {
		name          string
		securityGroup types.SecurityGroup
		expected      bool
	}{
		{
			name: "Security group in exception list",
			securityGroup: types.SecurityGroup{
				GroupId:   aws.String("sg-12345"),
				GroupName: aws.String("special-group"),
			},
			expected: true,
		},
		{
			name: "Security group not in exception list",
			securityGroup: types.SecurityGroup{
				GroupId:   aws.String("sg-67890"),
				GroupName: aws.String("my-security-group"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSecurityGroupException(tt.securityGroup)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasRules(t *testing.T) {
	tests := []struct {
		name          string
		securityGroup types.SecurityGroup
		expected      bool
	}{
		{
			name: "Security group with ingress rules",
			securityGroup: types.SecurityGroup{
				GroupId:   aws.String("sg-12345"),
				GroupName: aws.String("test-group"),
				IpPermissions: []types.IpPermission{
					{
						IpProtocol: aws.String("tcp"),
						FromPort:   aws.Int32(22),
						ToPort:     aws.Int32(22),
					},
				},
			},
			expected: true,
		},
		{
			name: "Security group with egress rules",
			securityGroup: types.SecurityGroup{
				GroupId:   aws.String("sg-67890"),
				GroupName: aws.String("test-group"),
				IpPermissionsEgress: []types.IpPermission{
					{
						IpProtocol: aws.String("-1"),
					},
				},
			},
			expected: true,
		},
		{
			name: "Security group with no rules",
			securityGroup: types.SecurityGroup{
				GroupId:   aws.String("sg-abcde"),
				GroupName: aws.String("test-group"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasRules(tt.securityGroup)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHandleSecurityGroup(t *testing.T) {
	// Save original dry run setting and restore after test
	originalDryRun := cfg.dryRun
	defer func() { cfg.dryRun = originalDryRun }()

	tests := []struct {
		name                 string
		securityGroup        types.SecurityGroup
		dryRun               bool
		hasNetworkInterfaces bool
		expectDelete         bool
		expectRevokeRules    bool
	}{
		{
			name: "Default security group - should not delete",
			securityGroup: types.SecurityGroup{
				GroupId:   aws.String("sg-12345"),
				GroupName: aws.String("default"),
			},
			dryRun:               false,
			hasNetworkInterfaces: false,
			expectDelete:         false,
			expectRevokeRules:    false,
		},
		{
			name: "Security group with network interfaces - should not delete",
			securityGroup: types.SecurityGroup{
				GroupId:   aws.String("sg-67890"),
				GroupName: aws.String("test-group"),
			},
			dryRun:               false,
			hasNetworkInterfaces: true,
			expectDelete:         false,
			expectRevokeRules:    false,
		},
		{
			name: "Unused security group with rules - should delete and revoke rules",
			securityGroup: types.SecurityGroup{
				GroupId:   aws.String("sg-abcde"),
				GroupName: aws.String("test-group"),
				IpPermissions: []types.IpPermission{
					{
						IpProtocol: aws.String("tcp"),
						FromPort:   aws.Int32(22),
						ToPort:     aws.Int32(22),
					},
				},
				IpPermissionsEgress: []types.IpPermission{
					{
						IpProtocol: aws.String("-1"),
					},
				},
			},
			dryRun:               false,
			hasNetworkInterfaces: false,
			expectDelete:         true,
			expectRevokeRules:    true,
		},
		{
			name: "Dry run - should not actually delete",
			securityGroup: types.SecurityGroup{
				GroupId:   aws.String("sg-abcde"),
				GroupName: aws.String("test-group"),
			},
			dryRun:               true,
			hasNetworkInterfaces: false,
			expectDelete:         false,
			expectRevokeRules:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.dryRun = tt.dryRun

			// Create mock client
			mockClient := &mockEC2Client{
				describeNetworkInterfacesOutput: &ec2.DescribeNetworkInterfacesOutput{
					NetworkInterfaces: make([]types.NetworkInterface, 0),
				},
				describeSecurityGroupsOutput: &ec2.DescribeSecurityGroupsOutput{
					SecurityGroups: []types.SecurityGroup{tt.securityGroup},
				},
			}

			if tt.hasNetworkInterfaces {
				mockClient.describeNetworkInterfacesOutput.NetworkInterfaces = append(
					mockClient.describeNetworkInterfacesOutput.NetworkInterfaces,
					types.NetworkInterface{
						NetworkInterfaceId: aws.String("eni-12345"),
					},
				)
			}

			// Create worker
			w := worker{
				id:                       1,
				wg:                       &sync.WaitGroup{},
				deletedSecurityGroupChan: make(chan string, 1),
			}

			// Call the function
			err := w.handleSecurityGroup(context.Background(), mockClient, tt.securityGroup)

			// Check results
			assert.NoError(t, err)
			assert.Equal(t, tt.expectDelete && !tt.dryRun, mockClient.deleteSecurityGroupCalled)
			assert.Equal(t, tt.expectRevokeRules && !tt.dryRun, mockClient.revokeIngressCalled || mockClient.revokeEgressCalled)

			// Clean up channel
			close(w.deletedSecurityGroupChan)
		})
	}
}
