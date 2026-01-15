// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/mock"
)

const (
	testProfile = "default"
	testRegion  = "us-east-1"
	testRoleARN = "arn:aws:iam::012345678912:role/XXXXXXXX"
)

var (
	// These are examples credentials pulled from:
	// https://docs.aws.amazon.com/STS/latest/APIReference/API_GetAccessKeyInfo.html
	testCredentials = aws.Credentials{
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		SessionToken:    "SessionToken",
	}
	testAWSConfig = aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider(testCredentials.AccessKeyID, testCredentials.SecretAccessKey, testCredentials.SessionToken),
	}
)

type mockCredentialsProvider struct {
	mock.Mock
}

var _ aws.CredentialsProvider = (*mockCredentialsProvider)(nil)

func (m *mockCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	args := m.Called(ctx)
	return args.Get(0).(aws.Credentials), args.Error(1)
}

type mockAssumeRoleClient struct {
	mock.Mock
}

var _ stscreds.AssumeRoleAPIClient = (*mockAssumeRoleClient)(nil)

func (m *mockAssumeRoleClient) AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sts.AssumeRoleOutput), args.Error(1)
}
