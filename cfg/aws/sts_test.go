// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func TestGetFallbackRegion(t *testing.T) {
	testCases := []struct {
		region string
		want   string
	}{
		{region: "us-east-1", want: classicFallbackRegion},
		{region: "us-west-2", want: classicFallbackRegion},
		{region: "eu-west-1", want: classicFallbackRegion},
		{region: "cn-north-1", want: bjsFallbackRegion},
		{region: "cn-northwest-1", want: bjsFallbackRegion},
		{region: "us-gov-east-1", want: pdtFallbackRegion},
		{region: "us-gov-west-1", want: pdtFallbackRegion},
		{region: "us-iso-east-1", want: dcaFallbackRegion},
		{region: "us-isob-east-1", want: lckFallbackRegion},
		{region: "unknown-region", want: classicFallbackRegion},
	}

	for _, testCase := range testCases {
		got := getFallbackRegion(testCase.region)
		assert.Equal(t, testCase.want, got)
	}
}

func TestStsCredentialsProvider_Retrieve(t *testing.T) {
	t.Run("Regional", func(t *testing.T) {
		regional := new(mockCredentialsProvider)
		regional.On("Retrieve", t.Context()).Return(testCredentials, nil).Once()
		partitional := new(mockCredentialsProvider)

		provider := &stsCredentialsProvider{
			regional:    regional,
			partitional: partitional,
		}

		got, err := provider.Retrieve(t.Context())
		require.NoError(t, err)
		assert.Equal(t, testCredentials, got)
		regional.AssertExpectations(t)
		partitional.AssertNotCalled(t, "Retrieve", t.Context())
	})

	t.Run("Regional/OtherError", func(t *testing.T) {
		regional := new(mockCredentialsProvider)
		regional.On("Retrieve", t.Context()).Return(aws.Credentials{}, assert.AnError).Once()
		partitional := new(mockCredentialsProvider)
		provider := &stsCredentialsProvider{
			regional:    regional,
			partitional: partitional,
		}

		_, err := provider.Retrieve(t.Context())
		assert.Equal(t, assert.AnError, err)
		regional.AssertExpectations(t)
		partitional.AssertNotCalled(t, "Retrieve", t.Context())
	})

	t.Run("Fallback/RegionDisabledException", func(t *testing.T) {
		regional := new(mockCredentialsProvider)
		regional.On("Retrieve", t.Context()).Return(aws.Credentials{}, &types.RegionDisabledException{}).Once()
		partitional := new(mockCredentialsProvider)
		partitional.On("Retrieve", t.Context()).Return(testCredentials, nil)

		provider := &stsCredentialsProvider{
			regional:    regional,
			partitional: partitional,
		}

		assert.Nil(t, provider.fallback)

		got, err := provider.Retrieve(t.Context())
		require.NoError(t, err)
		assert.Equal(t, testCredentials, got)
		assert.NotNil(t, provider.fallback)

		// Second call should use fallback directly
		got, err = provider.Retrieve(t.Context())
		require.NoError(t, err)
		assert.Equal(t, testCredentials, got)

		regional.AssertExpectations(t)
		partitional.AssertExpectations(t)
	})
}

func TestNewStsCredentialsProvider(t *testing.T) {
	provider := newStsCredentialsProvider(aws.Config{}, testRoleARN, testRegion)

	assert.NotNil(t, provider)
	stsProvider, ok := provider.(*stsCredentialsProvider)
	require.True(t, ok)
	assert.NotNil(t, stsProvider.regional)
	assert.NotNil(t, stsProvider.partitional)

	_, ok = stsProvider.regional.(*stscreds.AssumeRoleProvider)
	require.True(t, ok)
}

func TestConfusedDeputyHeaders(t *testing.T) {
	testCases := map[string]struct {
		envSourceArn          string
		envSourceAccount      string
		expectedHeaderArn     string
		expectedHeaderAccount string
	}{
		"unpopulated": {
			envSourceArn:          "",
			envSourceAccount:      "",
			expectedHeaderArn:     "",
			expectedHeaderAccount: "",
		},
		"both populated": {
			envSourceArn:          testInstanceARN,
			envSourceAccount:      testAccountID,
			expectedHeaderArn:     testInstanceARN,
			expectedHeaderAccount: testAccountID,
		},
		"only source arn populated": {
			envSourceArn:          testInstanceARN,
			envSourceAccount:      "",
			expectedHeaderArn:     "",
			expectedHeaderAccount: "",
		},
		"only source account populated": {
			envSourceArn:          "",
			envSourceAccount:      testAccountID,
			expectedHeaderArn:     "",
			expectedHeaderAccount: "",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(envconfig.AmzSourceAccount, testCase.envSourceAccount)
			t.Setenv(envconfig.AmzSourceArn, testCase.envSourceArn)

			client := newStsClient(testAWSConfig)

			// We aren't going to actually make the AssumeRole call, we are just going
			// to verify the headers are present once signed so the RoleArn and RoleSessionName
			// arguments are irrelevant. Fill them out with something so the request is valid.
			input := &sts.AssumeRoleInput{
				RoleArn:         aws.String(testRoleARN),
				RoleSessionName: aws.String("MockSession"),
			}

			// Use middleware to capture headers
			var capturedHeaders http.Header
			_, err := client.AssumeRole(t.Context(), input, func(o *sts.Options) {
				o.APIOptions = append(o.APIOptions, func(s *middleware.Stack) error {
					return s.Finalize.Add(middleware.FinalizeMiddlewareFunc("CaptureHeaders", func(_ context.Context, in middleware.FinalizeInput, _ middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
						if req, ok := in.Request.(*smithyhttp.Request); ok {
							capturedHeaders = req.Header.Clone()
						}
						// Don't actually send the request
						return middleware.FinalizeOutput{Result: &sts.AssumeRoleOutput{}}, middleware.Metadata{}, nil
					}), middleware.After)
				})
			})
			require.NoError(t, err)

			headerSourceArn := capturedHeaders.Get(SourceArnHeaderKey)
			assert.Equal(t, testCase.expectedHeaderArn, headerSourceArn)

			headerSourceAccount := capturedHeaders.Get(SourceAccountHeaderKey)
			assert.Equal(t, testCase.expectedHeaderAccount, headerSourceAccount)
		})
	}
}
