// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package retryer

import (
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func Test_IMDSRetryer_ShouldRetry(t *testing.T) {
	tests := []struct {
		name string
		req  *request.Request
		want bool
	}{
		{
			name: "ErrorIsNilDoNotRetry",
			req: &request.Request{
				Error: nil,
			},
			want: false,
		},
		{
			// no enum for status codes in request.Request nor http.Response
			name: "ErrorIsDefaultRetryable",
			req: &request.Request{
				Error: awserr.New("throttle me for 503", "throttle me for 503", nil),
				HTTPResponse: &http.Response{
					StatusCode: 503,
				},
			},
			want: true,
		},
		{
			name: "ErrorIsEC2MetadataErrorRetryable",
			req: &request.Request{
				Error: awserr.New("EC2MetadataError", "EC2MetadataError", nil),
			},
			want: true,
		},
		{
			name: "ErrorIsAWSOtherErrorNotRetryable",
			req: &request.Request{
				Error: awserr.New("other", "other", nil),
			},
			want: false,
		},
		{
			// errors.New as a parent error will always retry due to fallback
			name: "ErrorIsAWSOtherWithParentErrorRetryable",
			req: &request.Request{
				Error: awserr.New("other", "other", errors.New("other")),
			},
			want: true,
		},
		{
			// errors.New will always retry due to fallback
			name: "ErrorIsOtherErrorRetryable",
			req: &request.Request{
				Error: errors.New("other"),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewIMDSRetryer(GetDefaultRetryNumber()).ShouldRetry(tt.req); got != tt.want {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNumberOfRetryTest(t *testing.T) {
	tests := []struct {
		name               string
		expectedRetries    string
		expectedRetriesInt int
	}{
		{
			name:               "expect default for empty",
			expectedRetries:    "",
			expectedRetriesInt: DefaultImdsRetries,
		},
		{
			name:               "expect 2 for 2",
			expectedRetries:    "2",
			expectedRetriesInt: 2,
		},
		{
			name:               "expect default for invalid",
			expectedRetries:    "-1",
			expectedRetriesInt: 1,
		},
		{
			name:               "expect default for not int",
			expectedRetries:    "not an int",
			expectedRetriesInt: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer t.Setenv(envconfig.IMDS_NUMBER_RETRY, "")
			t.Setenv(envconfig.IMDS_NUMBER_RETRY, tt.expectedRetries)
			defaultIMDSRetryer := NewIMDSRetryer(GetDefaultRetryNumber())
			newIMDSRetryer := NewIMDSRetryer(tt.expectedRetriesInt)
			assert.Equal(t, defaultIMDSRetryer.MaxRetries(), tt.expectedRetriesInt)
			assert.Equal(t, newIMDSRetryer.MaxRetries(), tt.expectedRetriesInt)
		})
	}
}
