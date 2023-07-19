// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package retryer

import (
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
)

// IMDSRetryer this must implement request.Retryer
// not sure how to make in a test context not to retry
// this seems to only be an issue on mac
// windows and linux do not have this issue
// in text context do not try to retry
// this causes timeout failures for mac unit tests
// currently we set the var to nil in tests to mock
var IMDSRetryer request.Retryer = newIMDSRetryer()

type iMDSRetryer struct {
	client.DefaultRetryer
}

// newIMDSRetryer allows us to retry imds errors
// 2 imds calls 1 for hostname 1 for doc
// 2 calls 1 for imdsv2 1 for imdsv1
// 2 seconds 4 seconds 8 seconds 16 seconds 32 seconds = 1 minute 2 seconds
// total is 4 minutes 8 seconds
// random jitter is applied of half the retry time
// max retry total time is 6 minutes 12 seconds
// min retry total time is 2 minutes 4 seconds
func newIMDSRetryer() iMDSRetryer {
	return iMDSRetryer{
		DefaultRetryer: client.DefaultRetryer{
			NumMaxRetries: 5,
			MinRetryDelay: 2 * time.Second,
		},
	}
}

func (r iMDSRetryer) ShouldRetry(req *request.Request) bool {
	// there is no enum of error codes
	// EC2MetadataError is not retryable by default
	// Fallback to SDK's built in retry rules
	shouldRetry := false
	if awsError, ok := req.Error.(awserr.Error); r.DefaultRetryer.ShouldRetry(req) || (ok && awsError != nil && awsError.Code() == "EC2MetadataError") {
		shouldRetry = true
	}
	log.Printf("D! should retry %t for imds error : %v", shouldRetry, req.Error)
	return shouldRetry
}
