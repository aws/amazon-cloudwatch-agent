// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"os"
	"path/filepath"
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

		content, err := os.ReadFile(filepath.Join("testdata", "credential_original"))
		require.NoError(t, err)
		//nolint:gosec // G703: test-controlled temp file
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
				Profile:  "P",
				Filename: "F",
			},
			wantProvider: RefreshableSharedCredentialsProvider{},
		},
		// A half-configured static pair must select the static provider (which then fails on
		// retrieval) rather than silently falling through to a different identity.
		"StaticHalfPair": {
			cfg: &CredentialsConfig{
				AccessKey: "A",
				Profile:   "P",
				Filename:  "F",
			},
			wantProvider: credentials.StaticCredentialsProvider{},
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

func TestDefaultCredentialsChain_StaticHalfPairFailsLoudly(t *testing.T) {
	for name, cfg := range map[string]*CredentialsConfig{
		"access_key_only": {AccessKey: "A"},
		"secret_key_only": {SecretKey: "S"},
	} {
		t.Run(name, func(t *testing.T) {
			provider := cfg.fromChain()
			require.NotNil(t, provider)
			_, err := provider.Retrieve(t.Context())
			var emptyErr *credentials.StaticCredentialsEmptyError
			assert.ErrorAs(t, err, &emptyErr)
		})
	}
}

// The SDK falls back to its default ~/.aws/config whenever SharedConfigFiles is nil, and
// otherwise honors AWS_CONFIG_FILE via EnvConfig. Either way the shared config file must not
// participate in resolution unless AWS_SDK_LOAD_CONFIG is truthy.
func TestCredentialsConfig_LoadConfig_SharedConfigGate(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config")
	require.NoError(t, os.WriteFile(cfgFile, []byte(
		"[default]\nregion = eu-west-3\naws_access_key_id = AKIDFROMCONFIGFILE\n"+
			"aws_secret_access_key = secretFromConfigFile\n"), 0o600))

	for _, tc := range []struct {
		name, gate  string
		wantHonored bool
	}{
		{"unset", "", false}, {"false", "false", false}, {"true", "true", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{
				"AWS_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION",
				"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
			} {
				t.Setenv(k, "")
			}
			t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
			t.Setenv(envAwsSharedConfigFile, cfgFile)
			t.Setenv(envAwsSharedCredentialsFile, filepath.Join(dir, "no-such-credentials"))
			t.Setenv(envAwsSdkLoadConfig, tc.gate)

			cfg, err := (&CredentialsConfig{}).LoadConfig(t.Context())
			require.NoError(t, err)
			creds, credErr := cfg.Credentials.Retrieve(t.Context())
			if tc.wantHonored {
				require.NoError(t, credErr)
				assert.Equal(t, "AKIDFROMCONFIGFILE", creds.AccessKeyID)
				assert.Equal(t, "eu-west-3", cfg.Region)
				return
			}
			assert.Empty(t, cfg.Region, "region must not come from the shared config file")
			assert.NotEqual(t, "AKIDFROMCONFIGFILE", creds.AccessKeyID)
		})
	}
}
