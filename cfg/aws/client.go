// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
)

var (
	sharedHTTPClient     aws.HTTPClient
	sharedHTTPClientOnce sync.Once
)

// getSharedHTTPClient returns a singleton HTTP client for all AWS SDK operations. Sharing the client enables connection
// pooling and reuse across all AWS API calls, which reduces memory and file descriptor usage. It has to be an
// BuildableClient because the SDK will only append custom CA bundles if the client is of that type.
// https://github.com/aws/aws-sdk-go-v2/blob/v1.41.1/config/resolve.go#L57
func getSharedHTTPClient() aws.HTTPClient {
	sharedHTTPClientOnce.Do(func() {
		sharedHTTPClient = awshttp.NewBuildableClient().WithTimeout(1 * time.Minute)
	})
	return sharedHTTPClient
}
