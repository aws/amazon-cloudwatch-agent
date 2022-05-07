// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

type Refreshable_shared_credentials_provider struct {
	credentials.Expiry
	sharedCredentialsProvider *credentials.SharedCredentialsProvider

	// Retrival frequency, if the value is 15 minutes, the credentials will be retrieved every 15 minutes.
	ExpiryWindow time.Duration
}

// Retrieve reads and extracts the shared credentials from the current
// users home directory.
func (p *Refreshable_shared_credentials_provider) Retrieve() (credentials.Value, error) {

	p.SetExpiration(time.Now().Add(p.ExpiryWindow), 0)
	creds, err := p.sharedCredentialsProvider.Retrieve()

	return creds, err
}
