// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFallbackSharedConfigFiles(t *testing.T) {
	noOpGetUserHomeDir := func() string { return "home" }
	t.Setenv(envAwsSdkLoadConfig, "true")
	t.Setenv(envAwsSharedCredentialsFile, "credentials")
	t.Setenv(envAwsSharedConfigFile, "config")

	credFiles, cfgFiles := getFallbackSharedConfigFiles(noOpGetUserHomeDir)
	assert.Equal(t, []string{"credentials"}, credFiles)
	assert.Equal(t, []string{"config"}, cfgFiles)

	// Gate off: a non-nil empty config list keeps the SDK from loading the default ~/.aws/config.
	t.Setenv(envAwsSdkLoadConfig, "false")
	credFiles, cfgFiles = getFallbackSharedConfigFiles(noOpGetUserHomeDir)
	assert.Equal(t, []string{"credentials"}, credFiles)
	assert.NotNil(t, cfgFiles)
	assert.Empty(t, cfgFiles)

	t.Setenv(envAwsSdkLoadConfig, "")
	_, cfgFiles = getFallbackSharedConfigFiles(noOpGetUserHomeDir)
	assert.NotNil(t, cfgFiles)
	assert.Empty(t, cfgFiles)

	t.Setenv(envAwsSdkLoadConfig, "true")
	t.Setenv(envAwsSharedCredentialsFile, "")
	t.Setenv(envAwsSharedConfigFile, "")
	credFiles, cfgFiles = getFallbackSharedConfigFiles(noOpGetUserHomeDir)
	assert.Equal(t, []string{defaultSharedCredentialsFile("home")}, credFiles)
	assert.Equal(t, []string{defaultSharedConfig("home")}, cfgFiles)
}

// The SDK substitutes its default ~/.aws/config whenever SharedConfigFiles is nil, so the
// load options must always carry an explicit (possibly empty) list.
func TestLoadConfigOptions_SharedConfigFilesAlwaysExplicit(t *testing.T) {
	t.Setenv(envAwsSharedCredentialsFile, "credentials")
	t.Setenv(envAwsSharedConfigFile, "config")

	for _, gate := range []string{"", "false", "true"} {
		t.Run("gate="+gate, func(t *testing.T) {
			t.Setenv(envAwsSdkLoadConfig, gate)
			var lo config.LoadOptions
			for _, opt := range (&CredentialsConfig{}).loadOptions(nil) {
				require.NoError(t, opt(&lo))
			}
			assert.Equal(t, []string{"credentials"}, lo.SharedCredentialsFiles)
			assert.NotNil(t, lo.SharedConfigFiles)
			if gate == "true" {
				assert.Equal(t, []string{"config"}, lo.SharedConfigFiles)
			} else {
				assert.Empty(t, lo.SharedConfigFiles)
			}
		})
	}
}
