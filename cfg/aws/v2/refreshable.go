// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

type RefreshableSharedCredentialsProvider struct {
	// Path to the shared credentials file.
	Filename string
	// AWS Profile to extract credentials from the shared credentials file.
	Profile string
	// Retrieval frequency, if the value is 15 minutes, the credentials will be retrieved every 15 minutes.
	ExpiryWindow time.Duration
}

var _ aws.CredentialsProvider = (*RefreshableSharedCredentialsProvider)(nil)

// Retrieve reads and extracts the shared credentials from the current
// users home directory.
func (p RefreshableSharedCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	sharedConfig, err := config.LoadSharedConfigProfile(ctx, p.Profile, func(options *config.LoadSharedConfigOptions) {
		options.CredentialsFiles = []string{p.Filename}
	})
	if err != nil {
		return aws.Credentials{}, err
	}
	credentials := sharedConfig.Credentials
	credentials.CanExpire = true
	credentials.Expires = time.Now().Add(p.ExpiryWindow)
	return credentials, nil
}
