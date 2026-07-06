// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

func TestOIDCTokenFilePath(t *testing.T) {
	// The path follows the target platform, not the runtime OS, so translation is deterministic on any build host.
	assert.Equal(t, linuxOIDCTokenFile, OIDCTokenFilePath(config.OS_TYPE_LINUX))
	assert.Equal(t, linuxOIDCTokenFile, OIDCTokenFilePath(config.OS_TYPE_DARWIN))
	t.Setenv(ProgramData, "C:\\ProgramData")
	assert.Equal(t, "C:\\ProgramData\\Amazon\\AmazonCloudWatchAgent\\.oidc-token", OIDCTokenFilePath(config.OS_TYPE_WINDOWS))
}
