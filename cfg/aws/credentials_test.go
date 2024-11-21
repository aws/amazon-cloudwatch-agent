// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockConfigProvider struct{}

func (m mockConfigProvider) ClientConfig(serviceName string, cfgs ...*aws.Config) client.Config {
	return client.Config{
		Config: &aws.Config{
			// These are examples credentials pulled from:
			// https://docs.aws.amazon.com/STS/latest/APIReference/API_GetAccessKeyInfo.html
			Credentials: credentials.NewStaticCredentials("AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", ""),
			Region:      aws.String("us-east-1"),
		},
	}
}

func TestConfusedDeputyHeaders(t *testing.T) {
	mockProvider := mockConfigProvider{}

	tests := []struct {
		name                  string
		envSourceArn          string
		envSourceAccount      string
		expectedHeaderArn     string
		expectedHeaderAccount string
	}{
		{
			name:                  "unpopulated",
			envSourceArn:          "",
			envSourceAccount:      "",
			expectedHeaderArn:     "",
			expectedHeaderAccount: "",
		},
		{
			name:                  "both populated",
			envSourceArn:          "arn:aws:ec2:us-east-1:474668408639:instance/i-08293cd9825754f7c",
			envSourceAccount:      "539247453986",
			expectedHeaderArn:     "arn:aws:ec2:us-east-1:474668408639:instance/i-08293cd9825754f7c",
			expectedHeaderAccount: "539247453986",
		},
		{
			name:                  "only source arn populated",
			envSourceArn:          "arn:aws:ec2:us-east-1:474668408639:instance/i-08293cd9825754f7c",
			envSourceAccount:      "",
			expectedHeaderArn:     "",
			expectedHeaderAccount: "",
		},
		{
			name:                  "only source account populated",
			envSourceArn:          "",
			envSourceAccount:      "539247453986",
			expectedHeaderArn:     "",
			expectedHeaderAccount: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set global variables which will get picked up by newStsClient
			sourceArn = tt.envSourceArn
			sourceAccount = tt.envSourceAccount

			client := newStsClient(mockProvider)

			// Generate the assume role request, but do not actually send it
			// We don't need this unit test making real AWS calls
			request, _ := client.AssumeRoleRequest(&sts.AssumeRoleInput{
				// We aren't going to actually make the assume role call, we are just going
				// to verify the headers are present once signed so the RoleArn and RoleSessionName
				// arguments are irrelevant. Fill them out with something so the request is valid.
				RoleArn:         aws.String("XXXXXXX"),
				RoleSessionName: aws.String("XXXXXXX"),
			})

			// Headers are generated after the request is signed (but before it's sent)
			err := request.Sign()
			require.NoError(t, err)

			headerSourceArn := request.HTTPRequest.Header.Get("x-amz-source-arn")
			assert.Equal(t, tt.expectedHeaderArn, headerSourceArn)

			headerSourceAccount := request.HTTPRequest.Header.Get("x-amz-source-account")
			assert.Equal(t, tt.expectedHeaderAccount, headerSourceAccount)
		})
	}

	sourceArn = ""
	sourceAccount = ""
}
