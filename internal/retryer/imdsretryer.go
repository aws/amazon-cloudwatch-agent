// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package retryer

import (
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

const (
	DefaultImdsRetries = 1
)

type IMDSRetryer struct {
	client.DefaultRetryer
}

// NewIMDSRetryer allows us to retry imds errors
// otel component layer retries should come from aws config settings
// translator layer should come from env vars see GetDefaultRetryNumber()
func NewIMDSRetryer(imdsRetries int) IMDSRetryer {
	fmt.Printf("I! imds retry client will retry %d times", imdsRetries)
	return IMDSRetryer{
		DefaultRetryer: client.DefaultRetryer{
			NumMaxRetries: imdsRetries,
		},
	}
}

func (r IMDSRetryer) ShouldRetry(req *request.Request) bool {
	// there is no enum of error codes
	// EC2MetadataError is not retryable by default
	// Fallback to SDK's built in retry rules
	shouldRetry := false
	if awsError, ok := req.Error.(awserr.Error); r.DefaultRetryer.ShouldRetry(req) || (ok && awsError != nil && awsError.Code() == "EC2MetadataError") {
		shouldRetry = true
	}
	fmt.Printf("D! should retry %t for imds error : %v", shouldRetry, req.Error)
	return shouldRetry
}

func GetDefaultRetryNumber() int {
	imdsRetryEnv := os.Getenv(envconfig.IMDS_NUMBER_RETRY)
	imdsRetry, err := strconv.Atoi(imdsRetryEnv)
	if err == nil && imdsRetry >= 0 {
		return imdsRetry
	}
	return DefaultImdsRetries
}
