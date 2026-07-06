// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

// linuxOIDCTokenFile is the OIDC token path shared by oidctoken (writer) and sigv4auth (reader) on Linux/Darwin (etc dir).
const linuxOIDCTokenFile = "/opt/aws/amazon-cloudwatch-agent/etc/.oidc-token"

// OIDCTokenFilePath returns the OIDC token file path for the target platform; single source of truth for oidctoken and sigv4auth.
func OIDCTokenFilePath(targetPlatform string) string {
	if targetPlatform == config.OS_TYPE_WINDOWS {
		return GetWindowsProgramDataPath() + "\\Amazon\\AmazonCloudWatchAgent\\.oidc-token"
	}
	return linuxOIDCTokenFile
}
