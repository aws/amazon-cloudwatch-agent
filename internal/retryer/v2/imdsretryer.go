// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package retryer

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

const (
	DefaultMetadataRetries = 1
)

type IMDSRetryer struct {
	// Embed the standard retryer for default behavior
	*retry.Standard
}

var _ aws.RetryerV2 = (*IMDSRetryer)(nil)

// NewIMDSRetryer allows us to retry IMDS errors
func NewIMDSRetryer(retries int) *IMDSRetryer {
	fmt.Printf("I! IMDS retry client will retry %d times", retries)
	return &IMDSRetryer{
		Standard: retry.NewStandard(func(options *retry.StandardOptions) {
			options.MaxAttempts = retries + 1 // MaxAttempts include the first attempt
		}),
	}
}

func (r *IMDSRetryer) IsErrorRetryable(err error) bool {
	// SDKv2 returns a ResponseError on request failure. Any of those errors is considered retryable.
	// https://github.com/aws/aws-sdk-go-v2/blob/dcbed91b6c6235022f15eda6ea526dbb91e1cb81/feature/ec2/imds/request_middleware.go#L185-L191
	var responseErr *smithyhttp.ResponseError
	return errors.As(err, &responseErr) || r.Standard.IsErrorRetryable(err)
}

func GetDefaultRetryNumber() int {
	imdsRetryEnv := os.Getenv(envconfig.IMDS_NUMBER_RETRY)
	imdsRetry, err := strconv.Atoi(imdsRetryEnv)
	if err == nil && imdsRetry >= 0 {
		return imdsRetry
	}
	return DefaultMetadataRetries
}
