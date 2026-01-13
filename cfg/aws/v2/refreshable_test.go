// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshableSharedCredentialsProvider(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "credential")
	require.NoError(t, err)
	tmpFilename := tmpFile.Name()
	require.NoError(t, tmpFile.Close())

	provider := &RefreshableSharedCredentialsProvider{
		Filename:     tmpFilename,
		Profile:      testProfile,
		ExpiryWindow: 500 * time.Millisecond,
	}

	// Test invalid credential file
	got, err := provider.Retrieve(t.Context())
	assert.Error(t, err)
	assert.Equal(t, aws.Credentials{}, got)

	// Write initial credentials
	content, err := os.ReadFile("../testdata/credential_original")
	require.NoError(t, err)
	err = os.WriteFile(tmpFilename, content, 0600)
	require.NoError(t, err)

	// First retrieval
	got, err = provider.Retrieve(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "o1rLD3ykKN09", got.SecretAccessKey)
	assert.False(t, got.Expired())

	// Wait a bit but not enough to expire
	time.Sleep(100 * time.Millisecond)
	assert.False(t, got.Expired(), "Expect credentials not to be expired.")

	// Rotate credentials file
	content, err = os.ReadFile("../testdata/credential_rotate")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpFile.Name(), content, 0600))

	// Wait for expiry
	time.Sleep(500 * time.Millisecond)
	assert.True(t, got.Expired(), "Expect credentials to be expired.")

	// Retrieve new credentials
	got, err = provider.Retrieve(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "o1rLDaaaccc", got.SecretAccessKey)
	assert.False(t, got.Expired(), "Expect new credentials not to be expired.")
}
