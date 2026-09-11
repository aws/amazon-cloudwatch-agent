// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"os"
	"path/filepath"
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
		Provider: SharedCredentialsProvider{
			Filename: tmpFilename,
			Profile:  testProfile,
		},
		ExpiryWindow: 500 * time.Millisecond,
	}

	// Test invalid credential file
	got, err := provider.Retrieve(t.Context())
	assert.Error(t, err)
	assert.Equal(t, aws.Credentials{}, got)

	// Write initial credentials
	content, err := os.ReadFile(filepath.Join("testdata", "credential_original"))
	require.NoError(t, err)
	//nolint:gosec // G703: test-controlled temp file
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
	content, err = os.ReadFile(filepath.Join("testdata", "credential_rotate"))
	require.NoError(t, err)
	//nolint:gosec // G703: test-controlled temp file
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

// TestSharedCredentialsProvider_KeylessProfile verifies a profile that exists but
// carries no static keys fails loudly instead of returning zero-value credentials.
func TestSharedCredentialsProvider_KeylessProfile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "credentials")
	//nolint:gosec // G703: test-controlled temp file
	require.NoError(t, os.WriteFile(tmp, []byte("[default]\nregion = us-west-2\n"), 0600))

	p := SharedCredentialsProvider{Filename: tmp, Profile: "default"}
	_, err := p.Retrieve(t.Context())
	require.ErrorContains(t, err, "does not contain static credentials")
}

// TestSharedCredentialsProvider_EmptyFilenameHonorsEnvFile verifies an empty
// Filename resolves via AWS_SHARED_CREDENTIALS_FILE like the v1 provider did.
func TestSharedCredentialsProvider_EmptyFilenameHonorsEnvFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "credentials")
	//nolint:gosec // G101,G703: test-only fake credentials in a test-controlled temp file
	require.NoError(t, os.WriteFile(tmp, []byte("[default]\naws_access_key_id = AKIDEXAMPLE\naws_secret_access_key = secretEXAMPLE\n"), 0600))
	t.Setenv(envAwsSharedCredentialsFile, tmp)

	p := SharedCredentialsProvider{}
	got, err := p.Retrieve(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "AKIDEXAMPLE", got.AccessKeyID)
}

// TestSharedCredentialsProvider_IgnoresSharedConfigFile verifies the configured
// credentials file is authoritative: a decoy ~/.aws/config-style file with a
// different profile must not be merged in.
func TestSharedCredentialsProvider_IgnoresSharedConfigFile(t *testing.T) {
	dir := t.TempDir()
	decoy := filepath.Join(dir, "config")
	//nolint:gosec // G101,G703: test-only fake credentials in a test-controlled temp file
	require.NoError(t, os.WriteFile(decoy, []byte("[profile other]\naws_access_key_id = DECOY\naws_secret_access_key = decoySecret\n"), 0600))
	t.Setenv(envAwsSharedConfigFile, decoy)

	tmp := filepath.Join(dir, "credentials")
	//nolint:gosec // G703: test-controlled temp file
	require.NoError(t, os.WriteFile(tmp, []byte("[default]\nregion = us-west-2\n"), 0600))

	// The profile exists only in the decoy config file; with isolation in place
	// the credentials file is authoritative and resolution fails loudly.
	p := SharedCredentialsProvider{Filename: tmp, Profile: "other"}
	_, err := p.Retrieve(t.Context())
	require.Error(t, err)
}
