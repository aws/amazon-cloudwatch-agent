// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

const (
	defaultExpiryWindow = 10 * time.Minute
)

// RefreshableSharedCredentialsProvider wraps a SharedCredentialsProvider and sets an expiration.
type RefreshableSharedCredentialsProvider struct {
	// Provider is the underlying SharedCredentialsProvider.
	Provider SharedCredentialsProvider
	// Retrieval frequency, if the value is 15 minutes, the credentials will be retrieved every 15 minutes.
	ExpiryWindow time.Duration
}

var _ aws.CredentialsProvider = (*RefreshableSharedCredentialsProvider)(nil)

func (p RefreshableSharedCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	credentials, err := p.Provider.Retrieve(ctx)
	if err != nil {
		return aws.Credentials{}, err
	}
	credentials.CanExpire = true
	credentials.Expires = time.Now().Add(p.ExpiryWindow)
	return credentials, nil
}

// SharedCredentialsProvider loads the credentials from a shared credential file and profile.
type SharedCredentialsProvider struct {
	// Filename is the path to the shared credentials file.
	Filename string
	// Profile is the AWS Profile to extract credentials from the shared credentials file.
	Profile string
}

var _ aws.CredentialsProvider = (*SharedCredentialsProvider)(nil)

func (p SharedCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	var opts []func(*config.LoadSharedConfigOptions)
	if p.Filename != "" {
		opts = append(opts, func(options *config.LoadSharedConfigOptions) {
			options.CredentialsFiles = []string{p.Filename}
		})
	}
	sharedConfig, err := config.LoadSharedConfigProfile(ctx, p.Profile, opts...)
	if err != nil {
		return aws.Credentials{}, err
	}
	credentials := sharedConfig.Credentials
	return credentials, nil
}
