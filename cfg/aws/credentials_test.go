// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func TestConfusedDeputyHeaders(t *testing.T) {
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

			t.Setenv(envconfig.AmzSourceAccount, tt.envSourceAccount)
			t.Setenv(envconfig.AmzSourceArn, tt.envSourceArn)

			client := newStsClient(mock.Session, &aws.Config{
				// These are examples credentials pulled from:
				// https://docs.aws.amazon.com/STS/latest/APIReference/API_GetAccessKeyInfo.html
				Credentials:          credentials.NewStaticCredentials("AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", ""),
				Region:               aws.String("us-east-1"),
				UseDualStackEndpoint: endpoints.DualStackEndpointStateEnabled,
			})

			request, _ := client.AssumeRoleRequest(&sts.AssumeRoleInput{
				// We aren't going to actually make the assume role call, we are just going
				// to verify the headers are present once signed so the RoleArn and RoleSessionName
				// arguments are irrelevant. Fill them out with something so the request is valid.
				RoleArn:         aws.String("arn:aws:iam::012345678912:role/XXXXXXXX"),
				RoleSessionName: aws.String("MockSession"),
			})

			// Headers are generated after the request is signed (but before it's sent)
			err := request.Sign()
			require.NoError(t, err)

			headerSourceArn := request.HTTPRequest.Header.Get(SourceArnHeaderKey)
			assert.Equal(t, tt.expectedHeaderArn, headerSourceArn)

			headerSourceAccount := request.HTTPRequest.Header.Get(SourceAccountHeaderKey)
			assert.Equal(t, tt.expectedHeaderAccount, headerSourceAccount)
		})
	}

}
