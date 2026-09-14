// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

const (
	envAwsSdkLoadConfig         = "AWS_SDK_LOAD_CONFIG"
	envAwsSharedCredentialsFile = "AWS_SHARED_CREDENTIALS_FILE" // nolint:gosec
	envAwsSharedConfigFile      = "AWS_CONFIG_FILE"
)

// getFallbackSharedConfigFiles follows the same logic as the AWS SDK but takes a getUserHomeDir
// function. The shared-credentials and shared-config lists are returned separately because the
// v2 SDK loads them with different section-format rules. The shared config file is consulted
// only when AWS_SDK_LOAD_CONFIG is truthy; otherwise the config list is non-nil and empty, since
// the SDK treats nil as "not set" and would load the default ~/.aws/config.
//
// Mirrors the fork's internal/aws/awsutil/shared_config.go.
func getFallbackSharedConfigFiles(userHomeDirProvider func() string) ([]string, []string) {
	var sharedCredentialsFile, sharedConfigFile string
	setFromEnvVal(&sharedCredentialsFile, envAwsSharedCredentialsFile)
	if sharedCredentialsFile == "" {
		sharedCredentialsFile = defaultSharedCredentialsFile(userHomeDirProvider())
	}
	credentialsFiles := []string{sharedCredentialsFile}

	// Must be non-nil: the SDK treats a nil list as "not set" and loads the default ~/.aws/config.
	configFiles := []string{}
	enableSharedConfig, _ := strconv.ParseBool(os.Getenv(envAwsSdkLoadConfig))
	if enableSharedConfig {
		setFromEnvVal(&sharedConfigFile, envAwsSharedConfigFile)
		if sharedConfigFile == "" {
			sharedConfigFile = defaultSharedConfig(userHomeDirProvider())
		}
		configFiles = []string{sharedConfigFile}
	}
	return credentialsFiles, configFiles
}

func setFromEnvVal(dst *string, keys ...string) {
	for _, k := range keys {
		if v := os.Getenv(k); len(v) != 0 {
			*dst = v
			break
		}
	}
}

func defaultSharedCredentialsFile(dir string) string {
	return filepath.Join(dir, ".aws", "credentials")
}

func defaultSharedConfig(dir string) string {
	return filepath.Join(dir, ".aws", "config")
}

// backwardsCompatibleUserHomeDir provides the home directory based on
// environment variables.
//
// Based on v1.44.106 of the AWS SDK.
func backwardsCompatibleUserHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// currentUserHomeDir attempts to use the environment variables before falling
// back on the current user's home directory.
//
// Based on v1.44.332 of the AWS SDK.
func currentUserHomeDir() string {
	var home string

	home = backwardsCompatibleUserHomeDir()
	if len(home) > 0 {
		return home
	}

	currUser, _ := user.Current()
	if currUser != nil {
		home = currUser.HomeDir
	}

	return home
}
