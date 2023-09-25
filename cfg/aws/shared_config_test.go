// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFallbackSharedConfigFiles(t *testing.T) {
	noOpGetUserHomeDir := func() string { return "home" }
	t.Setenv(envAwsSdkLoadConfig, "true")
	t.Setenv(envAwsSharedCredentialsFile, "credentials")
	t.Setenv(envAwsSharedConfigFile, "config")

	got := getFallbackSharedConfigFiles(noOpGetUserHomeDir)
	assert.Equal(t, []string{"config", "credentials"}, got)

	t.Setenv(envAwsSdkLoadConfig, "false")
	got = getFallbackSharedConfigFiles(noOpGetUserHomeDir)
	assert.Equal(t, []string{"credentials"}, got)

	t.Setenv(envAwsSdkLoadConfig, "true")
	t.Setenv(envAwsSharedCredentialsFile, "")
	t.Setenv(envAwsSharedConfigFile, "")

	got = getFallbackSharedConfigFiles(noOpGetUserHomeDir)
	assert.Equal(t, []string{defaultSharedConfig("home"), defaultSharedCredentialsFile("home")}, got)
}
