// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows
// +build !windows

package tocwconfig

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func TestCompleteConfigUnix(t *testing.T) {
	resetContext(t)
	t.Setenv("JMX_JAR_PATH", "../../packaging/opentelemetry-jmx-metrics.jar")
	testutil.SetPrometheusRemoteWriteTestingEnv(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	expectedEnvVars := map[string]string{
		"CWAGENT_USER_AGENT": "CUSTOM USER AGENT VALUE",
		"CWAGENT_LOG_LEVEL":  "DEBUG",
		"AWS_SDK_LOG_LEVEL":  "LogDebug",
	}

	// The translation needs to use the runtime.GOOS value in order to generate the proper configuration YAML,
	// so this is separate
	checkTranslation(t, "complete_linux_config", "linux", expectedEnvVars, "")
	checkTranslation(t, "complete_darwin_config", "darwin", nil, "")
}

func TestAMPConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	testutil.SetPrometheusRemoteWriteTestingEnv(t)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "amp_config_linux", "linux", expectedEnvVars, "")
	checkTranslation(t, "amp_config_linux", "darwin", nil, "")
}

func TestJMXConfigLinux(t *testing.T) {
	resetContext(t)
	t.Setenv("JMX_JAR_PATH", "../../packaging/opentelemetry-jmx-metrics.jar")
	context.CurrentContext().SetMode(config.ModeEC2)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "jmx_config_linux", "linux", expectedEnvVars, "")
}

func TestDeltaConfigLinux(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "delta_config_linux", "linux", expectedEnvVars, "")
	checkTranslation(t, "delta_config_linux", "darwin", nil, "")
}

func TestDropOriginConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "drop_origin_linux", "linux", expectedEnvVars, "")
}
