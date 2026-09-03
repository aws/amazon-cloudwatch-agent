// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package retryer

import (
	"errors"
	"net/http"
	"testing"

	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func TestIMDSRetryer_IsErrorRetryable(t *testing.T) {
	testCases := map[string]struct {
		err  error
		want bool
	}{
		"ErrorIsNilNotRetryable": {
			err:  nil,
			want: false,
		},
		"ErrorIsIMDSResponseErrorRetryable": {
			err: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{
					Response: &http.Response{
						StatusCode: 404,
					},
				},
				Err: errors.New("request to EC2 IMDS failed"),
			},
			want: true,
		},
		"ErrorIsIMDSResponseError5xxRetryable": {
			err: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{
					Response: &http.Response{
						StatusCode: 500,
					},
				},
				Err: errors.New("request to EC2 IMDS failed"),
			},
			want: true,
		},
		"ErrorIsWrappedIMDSResponseErrorRetryable": {
			err: errors.Join(
				errors.New("outer error"),
				&smithyhttp.ResponseError{
					Response: &smithyhttp.Response{
						Response: &http.Response{
							StatusCode: 503,
						},
					},
					Err: errors.New("request to EC2 IMDS failed"),
				},
			),
			want: true,
		},
		"ErrorIsGenericErrorNotRetryableByDefault": {
			err:  errors.New("some other error"),
			want: false, // Standard retryer doesn't treat generic errors as retryable by default
		},
	}

	retryer := NewIMDSRetryer(DefaultMetadataRetries)

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got := retryer.IsErrorRetryable(testCase.err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestIMDSRetryer_MaxAttempts(t *testing.T) {
	testCases := map[string]struct {
		retries int
		want    int
	}{
		"DefaultRetries": {
			retries: DefaultMetadataRetries,
			want:    DefaultMetadataRetries + 1,
		},
		"TwoRetries": {
			retries: 2,
			want:    3,
		},
		"ZeroRetries": {
			retries: 0,
			want:    1,
		},
		"FiveRetries": {
			retries: 5,
			want:    6,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			retryer := NewIMDSRetryer(testCase.retries)
			assert.Equal(t, testCase.want, retryer.MaxAttempts())
		})
	}
}

func TestGetDefaultRetryNumber(t *testing.T) {
	testCases := map[string]struct {
		envValue        string
		expectedRetries int
	}{
		"EmptyEnvUsesDefault": {
			expectedRetries: DefaultMetadataRetries,
		},
		"NegativeEnvUsesDefault": {
			envValue:        "-1",
			expectedRetries: DefaultMetadataRetries,
		},
		"InvalidEnvUsesDefault": {
			envValue:        "not an int",
			expectedRetries: DefaultMetadataRetries,
		},
		"ZeroEnvValue": {
			envValue:        "0",
			expectedRetries: 0,
		},
		"ValidEnvValue": {
			envValue:        "2",
			expectedRetries: 2,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(envconfig.IMDS_NUMBER_RETRY, testCase.envValue)

			assert.Equal(t, testCase.expectedRetries, GetDefaultRetryNumber())
		})
	}
}
