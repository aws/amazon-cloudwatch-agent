// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	expiredTestRoleName = "cwa-integ-assume-role-expired"
	activeTestRoleName  = "cwagent-integ-test-task-role-active"
)

type mockIamClient struct {
	mock.Mock
}

var _ iamClient = (*mockIamClient)(nil)

func (m *mockIamClient) ListRoles(ctx context.Context, input *iam.ListRolesInput, optFns ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	args := m.Called(ctx, input, optFns)
	return args.Get(0).(*iam.ListRolesOutput), args.Error(1)
}

func (m *mockIamClient) GetRole(ctx context.Context, input *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	args := m.Called(ctx, input, optFns)
	return args.Get(0).(*iam.GetRoleOutput), args.Error(1)
}

func (m *mockIamClient) DeleteRole(ctx context.Context, input *iam.DeleteRoleInput, optFns ...func(*iam.Options)) (*iam.DeleteRoleOutput, error) {
	args := m.Called(ctx, input, optFns)
	return args.Get(0).(*iam.DeleteRoleOutput), args.Error(1)
}

func (m *mockIamClient) ListAttachedRolePolicies(ctx context.Context, input *iam.ListAttachedRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	args := m.Called(ctx, input, optFns)
	return args.Get(0).(*iam.ListAttachedRolePoliciesOutput), args.Error(1)
}

func (m *mockIamClient) DetachRolePolicy(ctx context.Context, input *iam.DetachRolePolicyInput, optFns ...func(*iam.Options)) (*iam.DetachRolePolicyOutput, error) {
	args := m.Called(ctx, input, optFns)
	return args.Get(0).(*iam.DetachRolePolicyOutput), args.Error(1)
}

func (m *mockIamClient) ListInstanceProfilesForRole(ctx context.Context, input *iam.ListInstanceProfilesForRoleInput, optFns ...func(*iam.Options)) (*iam.ListInstanceProfilesForRoleOutput, error) {
	args := m.Called(ctx, input, optFns)
	return args.Get(0).(*iam.ListInstanceProfilesForRoleOutput), args.Error(1)
}

func (m *mockIamClient) RemoveRoleFromInstanceProfile(ctx context.Context, input *iam.RemoveRoleFromInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.RemoveRoleFromInstanceProfileOutput, error) {
	args := m.Called(ctx, input, optFns)
	return args.Get(0).(*iam.RemoveRoleFromInstanceProfileOutput), args.Error(1)
}

func (m *mockIamClient) DeleteInstanceProfile(ctx context.Context, input *iam.DeleteInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.DeleteInstanceProfileOutput, error) {
	args := m.Called(ctx, input, optFns)
	return args.Get(0).(*iam.DeleteInstanceProfileOutput), args.Error(1)
}

func TestDeleteRoles(t *testing.T) {
	expirationDate := getExpirationDate()

	ctx := context.Background()
	ignoredRole := types.Role{
		CreateDate: aws.Time(expirationDate.Add(-2 * time.Hour)),
		RoleName:   aws.String("does-not-match-any-prefix"),
		RoleLastUsed: &types.RoleLastUsed{
			LastUsedDate: aws.Time(expirationDate.Add(-1 * time.Hour)),
		},
	}
	expiredRole := types.Role{
		CreateDate: aws.Time(expirationDate.Add(-2 * time.Hour)),
		RoleName:   aws.String(expiredTestRoleName),
		RoleLastUsed: &types.RoleLastUsed{
			LastUsedDate: aws.Time(expirationDate.Add(-1 * time.Hour)),
		},
	}
	activeRole := types.Role{
		CreateDate: aws.Time(expirationDate.Add(-2 * time.Hour)),
		RoleName:   aws.String(activeTestRoleName),
		RoleLastUsed: &types.RoleLastUsed{
			LastUsedDate: aws.Time(expirationDate.Add(time.Hour)),
		},
	}
	testRoles := []types.Role{ignoredRole, expiredRole, activeRole}
	testPolicies := []types.AttachedPolicy{
		{
			PolicyArn:  aws.String("policy-arn"),
			PolicyName: aws.String("policy-name"),
		},
	}
	testProfile := types.InstanceProfile{
		InstanceProfileName: aws.String("instance-profile-name"),
		Roles:               []types.Role{expiredRole},
	}

	client := &mockIamClient{}
	client.On("ListRoles", ctx, &iam.ListRolesInput{}, mock.Anything).Return(&iam.ListRolesOutput{Roles: testRoles}, nil)
	client.On("GetRole", ctx, &iam.GetRoleInput{RoleName: aws.String(expiredTestRoleName)}, mock.Anything).Return(&iam.GetRoleOutput{Role: &expiredRole}, nil)
	client.On("GetRole", ctx, &iam.GetRoleInput{RoleName: aws.String(activeTestRoleName)}, mock.Anything).Return(&iam.GetRoleOutput{Role: &activeRole}, nil)
	client.On("DeleteRole", ctx, mock.Anything, mock.Anything).Return(&iam.DeleteRoleOutput{}, nil)
	client.On("ListAttachedRolePolicies", ctx, mock.Anything, mock.Anything).Return(&iam.ListAttachedRolePoliciesOutput{
		AttachedPolicies: testPolicies,
	}, nil)
	client.On("DetachRolePolicy", ctx, mock.Anything, mock.Anything).Return(&iam.DetachRolePolicyOutput{}, nil)
	client.On("ListInstanceProfilesForRole", ctx, &iam.ListInstanceProfilesForRoleInput{
		RoleName: aws.String(expiredTestRoleName),
	}, mock.Anything).Return(&iam.ListInstanceProfilesForRoleOutput{InstanceProfiles: []types.InstanceProfile{testProfile}}, nil)
	client.On("RemoveRoleFromInstanceProfile", ctx, &iam.RemoveRoleFromInstanceProfileInput{
		RoleName:            aws.String(expiredTestRoleName),
		InstanceProfileName: testProfile.InstanceProfileName,
	}, mock.Anything).Return(&iam.RemoveRoleFromInstanceProfileOutput{}, nil)
	client.On("DeleteInstanceProfile", ctx, &iam.DeleteInstanceProfileInput{
		InstanceProfileName: testProfile.InstanceProfileName,
	}, mock.Anything).Return(&iam.DeleteInstanceProfileOutput{}, nil)
	assert.NoError(t, deleteRoles(ctx, client, expirationDate))
	assert.Len(t, client.Calls, 9)
	for _, call := range client.Calls {
		switch call.Method {
		case "DeleteRole":
			input := call.Arguments.Get(1).(*iam.DeleteRoleInput)
			assert.Equal(t, expiredTestRoleName, *input.RoleName)
		case "ListAttachedRolePolicies":
			input := call.Arguments.Get(1).(*iam.ListAttachedRolePoliciesInput)
			assert.Equal(t, expiredTestRoleName, *input.RoleName)
		case "DetachRolePolicy":
			input := call.Arguments.Get(1).(*iam.DetachRolePolicyInput)
			assert.Equal(t, expiredTestRoleName, *input.RoleName)
			assert.Equal(t, "policy-arn", *input.PolicyArn)
		}
	}
}
