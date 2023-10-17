// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package tocwconfig

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func TestCompleteConfigWindows(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	expectedEnvVars := map[string]string{
		"CWAGENT_USER_AGENT": "CUSTOM USER AGENT VALUE",
		"CWAGENT_LOG_LEVEL":  "DEBUG",
		"AWS_SDK_LOG_LEVEL":  "LogDebug",
	}

	// The translation needs to use the runtime.GOOS value in order to generate the proper configuration YAML,
	// so this is separate
	checkTranslation(t, "complete_windows_config", "windows", expectedEnvVars, "")
}
