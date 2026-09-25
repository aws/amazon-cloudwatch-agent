// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
)

func TestSharedConfigRegion(t *testing.T) {
	const profile = "AmazonCloudWatchAgent"
	const (
		namedCreds   = "[" + profile + "]\naws_access_key_id = AKIATEST\naws_secret_access_key = secret\nregion = eu-central-1\n"
		defaultCreds = "[default]\nregion = us-west-1\n"
		namedConfig  = "[profile " + profile + "]\nregion = ap-south-1\n"
		partialCreds = "[" + profile + "]\naws_access_key_id = AKIATEST\nregion = eu-central-1\n" // secret missing
	)

	testCases := map[string]struct {
		env          map[string]string
		credsFile    string // content of an explicit credentials file; "" for none
		configFile   string // content of an explicit config file; "" for none
		defaultCreds string // content of the AWS_SHARED_CREDENTIALS_FILE target
		profile      string
		want         string
		wantErr      bool
	}{
		"Env/AWS_REGION":         {env: map[string]string{"AWS_REGION": "eu-west-1"}, profile: profile, want: "eu-west-1"},
		"Env/AWS_DEFAULT_REGION": {env: map[string]string{"AWS_DEFAULT_REGION": "eu-west-2"}, profile: profile, want: "eu-west-2"},
		"Env/BeatsFile":          {env: map[string]string{"AWS_REGION": "eu-west-1"}, credsFile: namedCreds, profile: profile, want: "eu-west-1"},
		// Differs from SDK v1, which aborted on any shared-file error: a broken profile is
		// reported at credential time, not as a missing region.
		"Env/BeatsBrokenProfile": {env: map[string]string{"AWS_REGION": "eu-west-1"}, credsFile: partialCreds, profile: profile, want: "eu-west-1"},

		"File/CredentialsSlot": {credsFile: namedCreds, profile: profile, want: "eu-central-1"},
		"File/ConfigSlot":      {configFile: namedConfig, profile: profile, want: "ap-south-1"},
		// SDK v2 merges credentials-file sections over config-file sections (SDK v1 read the
		// files in list order, so the config file won).
		"File/CredentialsWinsOverConfig": {credsFile: namedCreds, configFile: namedConfig, profile: profile, want: "eu-central-1"},
		// A bare "[name]" section is dropped from a file in the config slot.
		"File/ConfigSlotDropsBareSection": {configFile: namedCreds, profile: profile, want: ""},
		"File/OnlyGivenFilesAreRead":      {credsFile: namedCreds, defaultCreds: "[" + profile + "]\nregion = us-west-1\n", profile: profile, want: "eu-central-1"},
		"File/DefaultLocationWhenNil":     {defaultCreds: namedCreds, profile: profile, want: "eu-central-1"},

		"Profile/EmptyUsesAWS_PROFILE":         {env: map[string]string{"AWS_PROFILE": profile}, credsFile: defaultCreds + namedCreds, want: "eu-central-1"},
		"Profile/EmptyUsesAWS_DEFAULT_PROFILE": {env: map[string]string{"AWS_DEFAULT_PROFILE": profile}, credsFile: defaultCreds + namedCreds, want: "eu-central-1"},
		"Profile/EmptyUsesDefault":             {credsFile: defaultCreds + namedCreds, want: "us-west-1"},
		"Profile/MissingIsNotAnError":          {credsFile: defaultCreds, profile: profile, want: ""},

		"Nothing": {profile: profile, want: ""},
		// Differs from SDK v1, which ignored partial static credentials: SDK v2 rejects the
		// whole profile, so its region is unreadable.
		"BrokenProfileIsAnError": {credsFile: partialCreds, profile: profile, wantErr: true},
		"InvalidEnvIsAnError":    {env: map[string]string{"AWS_REGION": "eu-west-1", "AWS_MAX_ATTEMPTS": "not-a-number"}, profile: profile, wantErr: true},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			dir := testutil.IsolateAWSSharedConfigEnv(t)
			for k, v := range testCase.env {
				t.Setenv(k, v)
			}
			var credsFiles, configFiles []string
			if testCase.credsFile != "" {
				path := filepath.Join(dir, "credentials")
				require.NoError(t, os.WriteFile(path, []byte(testCase.credsFile), 0o600))
				credsFiles = []string{path}
			}
			if testCase.configFile != "" {
				path := filepath.Join(dir, "config")
				require.NoError(t, os.WriteFile(path, []byte(testCase.configFile), 0o600))
				configFiles = []string{path}
			}
			if testCase.defaultCreds != "" {
				require.NoError(t, os.WriteFile(os.Getenv(envAwsSharedCredentialsFile), []byte(testCase.defaultCreds), 0o600))
			}

			region, err := SharedConfigRegion(t.Context(), testCase.profile, credsFiles, configFiles)
			if testCase.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, testCase.want, region)
		})
	}
}
