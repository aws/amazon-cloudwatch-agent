// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

const (
	defaultExpiryWindow = 10 * time.Minute
	defaultProfileName  = "default"
	envAwsProfile       = "AWS_PROFILE"
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
// An empty Filename resolves like the v1 SDK: AWS_SHARED_CREDENTIALS_FILE when set, otherwise
// the default shared credentials file. An empty Profile resolves to AWS_PROFILE when set,
// otherwise "default" (the v2 SDK's LoadSharedConfigProfile rejects an empty profile name).
type SharedCredentialsProvider struct {
	// Filename is the path to the shared credentials file.
	Filename string
	// Profile is the AWS Profile to extract credentials from the shared credentials file.
	Profile string
}

var _ aws.CredentialsProvider = (*SharedCredentialsProvider)(nil)

func (p SharedCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	profile := p.Profile
	if profile == "" {
		profile = os.Getenv(envAwsProfile)
	}
	if profile == "" {
		profile = defaultProfileName
	}
	filename := p.Filename
	if filename == "" {
		setFromEnvVal(&filename, envAwsSharedCredentialsFile)
	}
	if filename == "" {
		filename = defaultSharedCredentialsFile(backwardsCompatibleUserHomeDir())
	}
	opts := []func(*config.LoadSharedConfigOptions){func(o *config.LoadSharedConfigOptions) {
		// Read credentials only from the resolved file. Empty ConfigFiles
		// prevents the SDK from also merging the default shared config file
		// (for example $HOME/.aws/config), so the credentials file is
		// authoritative and a missing file or profile fails loudly instead
		// of silently resolving elsewhere.
		o.CredentialsFiles = []string{filename}
		o.ConfigFiles = []string{}
	}}
	sharedConfig, err := config.LoadSharedConfigProfile(ctx, profile, opts...)
	if err != nil {
		return aws.Credentials{}, err
	}
	if !sharedConfig.Credentials.HasKeys() {
		return aws.Credentials{}, fmt.Errorf("shared credentials profile %q in %q does not contain static credentials", profile, filename)
	}
	return sharedConfig.Credentials, nil
}
