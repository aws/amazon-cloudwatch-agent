// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCredentialsConfig_LoadConfig(t *testing.T) {
	t.Run("FromStatic", func(t *testing.T) {
		config := &CredentialsConfig{
			Region:    testRegion,
			AccessKey: "StaticAccess",
			SecretKey: "StaticSecret",
		}

		cfg, err := config.LoadConfig(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "us-east-1", cfg.Region)
		assert.NotNil(t, cfg.Credentials)
		cache, ok := cfg.Credentials.(*aws.CredentialsCache)
		assert.True(t, ok)
		assert.True(t, cache.IsCredentialsProvider(credentials.StaticCredentialsProvider{}))
		got, err := cfg.Credentials.Retrieve(t.Context())
		assert.NoError(t, err)
		assert.Equal(t, "StaticAccess", got.AccessKeyID)
		assert.Equal(t, "StaticSecret", got.SecretAccessKey)
	})

	t.Run("FromRefreshable", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmpFile, err := os.CreateTemp(tmpDir, "credential")
		require.NoError(t, err)
		tmpFilename := tmpFile.Name()
		require.NoError(t, tmpFile.Close())

		content, err := os.ReadFile("../testdata/credential_original")
		require.NoError(t, err)
		err = os.WriteFile(tmpFilename, content, 0600)
		require.NoError(t, err)

		config := &CredentialsConfig{
			Region:   testRegion,
			Filename: tmpFilename,
			Profile:  testProfile,
		}

		cfg, err := config.LoadConfig(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "us-east-1", cfg.Region)
		assert.NotNil(t, cfg.Credentials)
		cache, ok := cfg.Credentials.(*aws.CredentialsCache)
		assert.True(t, ok)
		assert.True(t, cache.IsCredentialsProvider(RefreshableSharedCredentialsProvider{}))
		got, err := cfg.Credentials.Retrieve(t.Context())
		assert.NoError(t, err)
		assert.Equal(t, "ASIAIKJ", got.AccessKeyID)
		assert.Equal(t, "o1rLD3ykKN09", got.SecretAccessKey)
	})

	t.Run("FromRoleARN", func(t *testing.T) {
		original := newAssumeRoleClient
		t.Cleanup(func() {
			newAssumeRoleClient = original
		})

		marc := new(mockAssumeRoleClient)
		marc.On("AssumeRole", mock.Anything, mock.Anything, mock.Anything).Return(&sts.AssumeRoleOutput{
			Credentials: &types.Credentials{
				AccessKeyId:     aws.String("AssumedAccess"),
				SecretAccessKey: aws.String("AssumedSecret"),
				SessionToken:    aws.String("AssumedToken"),
				Expiration:      aws.Time(time.Now().Add(5 * time.Minute)),
			},
		}, nil).Once()
		newAssumeRoleClient = func(aws.Config) stscreds.AssumeRoleAPIClient {
			return marc
		}

		config := &CredentialsConfig{
			Region:    testRegion,
			AccessKey: "StaticAccess",
			SecretKey: "StaticSecret",
			RoleARN:   testRoleARN,
		}

		cfg, err := config.LoadConfig(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "us-east-1", cfg.Region)
		assert.NotNil(t, cfg.Credentials)
		cache, ok := cfg.Credentials.(*aws.CredentialsCache)
		assert.True(t, ok)
		assert.True(t, cache.IsCredentialsProvider(&stsCredentialsProvider{}))
		got, err := cfg.Credentials.Retrieve(t.Context())
		assert.NoError(t, err)
		assert.Equal(t, "AssumedAccess", got.AccessKeyID)
		assert.Equal(t, "AssumedSecret", got.SecretAccessKey)
		assert.Equal(t, "AssumedToken", got.SessionToken)
		marc.AssertExpectations(t)
	})
}

func TestOverwriteCredentialsChain(t *testing.T) {
	originalChain := CredentialsChain()
	t.Cleanup(func() {
		OverwriteCredentialsChain(originalChain...)
	})

	mcp := new(mockCredentialsProvider)
	customProvider := CredentialsProvider{
		Name: func() string { return "MockProvider" },
		Provider: func(c *CredentialsConfig) aws.CredentialsProvider {
			if c.Token != "" {
				return mcp
			}
			return nil
		},
	}

	OverwriteCredentialsChain(customProvider)
	chain := CredentialsChain()
	assert.Len(t, chain, 1)
	assert.Equal(t, "MockProvider", chain[0].Name())

	cfg := &CredentialsConfig{}
	provider := cfg.fromChain()
	assert.Nil(t, provider)
	cfg.Token = "T"
	provider = cfg.fromChain()
	assert.IsType(t, mcp, provider)
}

func TestDefaultCredentialsChain(t *testing.T) {
	testCases := map[string]struct {
		cfg          *CredentialsConfig
		wantProvider aws.CredentialsProvider
	}{
		"Static": {
			cfg: &CredentialsConfig{
				AccessKey: "A",
				SecretKey: "S",
				Token:     "T",
				Profile:   "P",
				Filename:  "F",
			},
			wantProvider: credentials.StaticCredentialsProvider{},
		},
		"Refreshable": {
			cfg: &CredentialsConfig{
				AccessKey: "A",
				Profile:   "P",
				Filename:  "F",
			},
			wantProvider: RefreshableSharedCredentialsProvider{},
		},
		"NotInChain": {
			cfg:          &CredentialsConfig{},
			wantProvider: nil,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			provider := testCase.cfg.fromChain()

			if testCase.wantProvider != nil {
				assert.NotNil(t, provider)
				cache, ok := provider.(*aws.CredentialsCache)
				assert.True(t, ok)
				assert.True(t, cache.IsCredentialsProvider(testCase.wantProvider))
			} else {
				assert.Nil(t, provider)
			}
		})
	}
}
