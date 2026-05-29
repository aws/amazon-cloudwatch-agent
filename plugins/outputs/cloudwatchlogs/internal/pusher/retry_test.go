// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	sdkhttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ httpResponseError = (*smithyhttp.ResponseError)(nil)
var _ httpResponseError = (*sdkhttp.ResponseError)(nil)

func TestChooseRetryWaitStrategy(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		err              error
		expectedStrategy retryWaitStrategy
	}{
		"ResourceNotFoundException": {
			err:              &types.ResourceNotFoundException{},
			expectedStrategy: retryShort,
		},
		"InvalidSequenceTokenException": {
			err:              &types.InvalidSequenceTokenException{},
			expectedStrategy: retryShort,
		},
		"ServiceUnavailableException": {
			err:              &types.ServiceUnavailableException{},
			expectedStrategy: retryLong,
		},
		"ThrottlingException": {
			err:              &types.ThrottlingException{},
			expectedStrategy: retryLong,
		},
		"500 - InternalFailure": {
			err: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{
					Response: &http.Response{
						StatusCode: 500,
					},
				},
				Err: errors.New("InternalFailure"),
			},
			expectedStrategy: retryLong,
		},
		"503 - ServiceUnavailable": {
			err: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{
					Response: &http.Response{
						StatusCode: 503,
					},
				},
				Err: errors.New("ServiceUnavailable"),
			},
			expectedStrategy: retryLong,
		},
		"Connection Refused": {
			err:              &smithy.GenericAPIError{Code: "SomeError", Message: "connection refused"},
			expectedStrategy: retryLong,
		},
		"Connection Refused - syscall": {
			err:              syscall.ECONNREFUSED,
			expectedStrategy: retryLong,
		},
		"Connection Reset By Peer": {
			err:              &smithy.GenericAPIError{Code: "SomeError", Message: "connection reset by peer"},
			expectedStrategy: retryLong,
		},
		"Connection Reset By Peer - syscall": {
			err:              syscall.ECONNRESET,
			expectedStrategy: retryLong,
		},
		"Connection Timeout": {
			err:              syscall.ETIMEDOUT,
			expectedStrategy: retryLong,
		},
		"Request Timeout": {
			err:              &smithy.GenericAPIError{Code: "RequestTimeout", Message: "request timed out"},
			expectedStrategy: retryLong,
		},
		"Response Timeout": {
			err:              &smithy.GenericAPIError{Code: "ResponseTimeout", Message: "response timed out"},
			expectedStrategy: retryLong,
		},
		"Deadline Exceeded": {
			err:              os.ErrDeadlineExceeded,
			expectedStrategy: retryLong,
		},
		"Other Errors": {
			err:              errors.New("Unknown Error"),
			expectedStrategy: retryShort,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			chosen := chooseRetryWaitStrategy(tt.err)
			require.Equal(t, tt.expectedStrategy, chosen)
		})
	}
}

func TestRetryWaitShort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		retryCount  int
		minDuration time.Duration
		maxDuration time.Duration
	}{
		{
			retryCount:  0,
			minDuration: 100 * time.Millisecond,
			maxDuration: 200 * time.Millisecond,
		},
		{
			retryCount:  1,
			minDuration: 200 * time.Millisecond,
			maxDuration: 400 * time.Millisecond,
		},
		{
			retryCount:  2,
			minDuration: 400 * time.Millisecond,
			maxDuration: 800 * time.Millisecond,
		},
		{
			retryCount:  3,
			minDuration: 800 * time.Millisecond,
			maxDuration: 1600 * time.Millisecond,
		},
		{
			retryCount:  4,
			minDuration: 1600 * time.Millisecond,
			maxDuration: 3200 * time.Millisecond,
		},
		{
			retryCount:  5,
			minDuration: 30 * time.Second,
			maxDuration: 1 * time.Minute,
		},
		{
			retryCount:  6,
			minDuration: 30 * time.Second,
			maxDuration: 1 * time.Minute,
		},
		{
			retryCount:  7,
			minDuration: 30 * time.Second,
			maxDuration: 1 * time.Minute,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.retryCount), func(t *testing.T) {
			for _ = range 1000 {
				duration := retryWaitShort(tt.retryCount)
				assert.GreaterOrEqual(t, duration, tt.minDuration, "retryWaitShort(%v) should be greater than or equal to %v", tt.retryCount, tt.minDuration)
				assert.LessOrEqual(t, duration, tt.maxDuration, "retryWaitShort(%v) should be less than or equal to %v", tt.retryCount, tt.maxDuration)
			}
		})
	}
}

func TestRetryWaitLong(t *testing.T) {
	t.Parallel()
	tests := []struct {
		retryCount  int
		minDuration time.Duration
		maxDuration time.Duration
	}{
		{
			retryCount:  0,
			minDuration: 1 * time.Second,
			maxDuration: 2 * time.Second,
		},
		{
			retryCount:  1,
			minDuration: 2 * time.Second,
			maxDuration: 4 * time.Second,
		},
		{
			retryCount:  2,
			minDuration: 30 * time.Second,
			maxDuration: 1 * time.Minute,
		},
		{
			retryCount:  3,
			minDuration: 30 * time.Second,
			maxDuration: 1 * time.Minute,
		},
		{
			retryCount:  4,
			minDuration: 30 * time.Second,
			maxDuration: 1 * time.Minute,
		},
		{
			retryCount:  5,
			minDuration: 30 * time.Second,
			maxDuration: 1 * time.Minute,
		},
		{
			retryCount:  6,
			minDuration: 30 * time.Second,
			maxDuration: 1 * time.Minute,
		},
		{
			retryCount:  7,
			minDuration: 30 * time.Second,
			maxDuration: 1 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.retryCount), func(t *testing.T) {
			for _ = range 1000 {
				duration := retryWaitLong(tt.retryCount)
				assert.GreaterOrEqual(t, duration, tt.minDuration, "retryWaitLong(%v) should be greater than or equal to %v", tt.retryCount, tt.minDuration)
				assert.LessOrEqual(t, duration, tt.maxDuration, "retryWaitLong(%v) should be less than or equal to %v", tt.retryCount, tt.maxDuration)
			}
		})
	}
}
