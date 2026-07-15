// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package tocwconfig

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
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

func TestWindowsEventsOtelConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	checkTranslation(t, "opentelemetry/windows_events_config", "windows", nil, "")
}

func TestDefaultOtelConfigWindowsTranslation(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	agent.Global_Config.Region = "us-west-2"
	agent.Global_Config.RegionType = config.RegionTypeCredsMap

	cfg, ok := config.DefaultJSONConfigFor("otel")
	require.True(t, ok)

	var input any
	require.NoError(t, json.Unmarshal([]byte(cfg), &input))

	translator.SetTargetPlatform("windows")
	verifyToTomlTranslation(t, input, "./sampleConfig/opentelemetry/default_otel_config_windows.conf")
	verifyToYamlTranslation(t, input, "./sampleConfig/opentelemetry/default_otel_config_windows.yaml")
}
