// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/stretchr/testify/assert"
)

func TestSharedCredentialsProviderExpiryWindowIsExpired(t *testing.T) {
	tmpFile, _ := os.CreateTemp(os.TempDir(), "credential")
	defer os.Remove(tmpFile.Name())
	bytes, _ := os.ReadFile("./testdata/credential_original")
	os.WriteFile(tmpFile.Name(), bytes, 0644)
	p := credentials.NewCredentials(&Refreshable_shared_credentials_provider{
		sharedCredentialsProvider: &credentials.SharedCredentialsProvider{
			Filename: tmpFile.Name(),
			Profile:  "",
		},
		ExpiryWindow: 1 * time.Second,
	})
	creds, _ := p.Get()
	assert.Equal(t, "o1rLD3ykKN09", creds.SecretAccessKey)
	time.Sleep(1 * time.Millisecond)

	assert.False(t, p.IsExpired(), "Expect creds not to be expired.")

	bytes_rotate, _ := os.ReadFile("./testdata/credential_rotate")
	os.WriteFile(tmpFile.Name(), bytes_rotate, 0644)

	time.Sleep(2 * time.Second)

	assert.True(t, p.IsExpired(), "Expect creds to be expired.")
	creds, _ = p.Get()
	assert.Equal(t, "o1rLDaaaccc", creds.SecretAccessKey)
	assert.False(t, p.IsExpired(), "Expect creds not to be expired.")

	time.Sleep(1 * time.Second)
	assert.True(t, p.IsExpired(), "Expect creds to be expired.")
}
