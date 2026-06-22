// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows
// +build !windows

package tocwconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/internal/mapstructure"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/totomlconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/toyamlconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	otel "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel"
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

func TestDualStackConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	testutil.SetPrometheusRemoteWriteTestingEnv(t)
	expectedEnvVars := map[string]string{
		"AWS_USE_DUALSTACK_ENDPOINT": "true",
	}
	checkTranslation(t, "dualstack_config", "linux", expectedEnvVars, "")
}

func TestJMXConfigLinux(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	testutil.SetPrometheusRemoteWriteTestingEnv(t)
	t.Setenv("JMX_JAR_PATH", "../../packaging/opentelemetry-jmx-metrics.jar")
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "jmx_config_linux", "linux", expectedEnvVars, "")
}

func TestJMXConfigEKS(t *testing.T) {
	resetContext(t)
	testutil.SetPrometheusRemoteWriteTestingEnv(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	context.CurrentContext().SetRunInContainer(true)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "jmx_eks_config_linux", "linux", expectedEnvVars, "")
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

func TestDBIConfigLinux(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	require.NoError(t, os.Chmod("sampleConfig/opentelemetry/testdata/.pgpass", 0600))
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "opentelemetry/dbi_config_linux", "linux", expectedEnvVars, "")
}

func TestJournaldLogsUnits(t *testing.T) {
	resetContext(t)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "journaldlogs_units", "linux", expectedEnvVars, "")
}

func TestJournaldLogsPriority(t *testing.T) {
	resetContext(t)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "journaldlogs_priority", "linux", expectedEnvVars, "")
}

func TestJournaldLogsFilters(t *testing.T) {
	resetContext(t)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "journaldlogs_filters", "linux", expectedEnvVars, "")
}

func TestJournaldLogsMatches(t *testing.T) {
	resetContext(t)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "journaldlogs_matches", "linux", expectedEnvVars, "")
}

func TestJournaldLogsUnitsAndPriority(t *testing.T) {
	resetContext(t)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "journaldlogs_units_and_priority", "linux", expectedEnvVars, "")
}

func TestCombinedV1V2EC2Config(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	checkTranslation(t, "opentelemetry/combined_v1_v2_ec2_config", "linux", nil, "")
}

func TestCombinedV1V2EKSConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	context.CurrentContext().SetKubernetesMode(config.ModeEKS)

	// Cannot use checkTranslation here because the container_insights prometheus
	// receiver references /var/run/secrets/kubernetes.io/serviceaccount/token
	// which only exists inside K8s pods. Translate without collector validation.
	agent.Global_Config = *new(agent.Agent)
	translator.SetTargetPlatform("linux")
	var input interface{}
	blob, err := os.ReadFile("./sampleConfig/opentelemetry/combined_v1_v2_eks_config.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(blob, &input))
	tomlConfig, err := cmdutil.TranslateJsonMapToTomlConfig(input)
	require.NoError(t, err)
	actualToml := totomlconfig.ToTomlConfig(tomlConfig)

	expectedConf, err := os.ReadFile("./sampleConfig/opentelemetry/combined_v1_v2_eks_config.conf")
	require.NoError(t, err)
	assert.Equal(t, string(expectedConf), actualToml)

	var expected interface{}
	bs, err := os.ReadFile("./sampleConfig/opentelemetry/combined_v1_v2_eks_config.yaml")
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(bs, &expected))

	var actual interface{}
	cfg, err := otel.TranslateWithoutValidation(input, context.CurrentContext().Os())
	require.NoError(t, err)
	yamlConfig, err := mapstructure.Marshal(cfg)
	require.NoError(t, err)
	yamlStr := toyamlconfig.ToYamlConfig(yamlConfig)
	require.NoError(t, yaml.Unmarshal([]byte(yamlStr), &actual))

	opt := cmpopts.SortSlices(func(x, y interface{}) bool {
		return fmt.Sprintf("%v", x) < fmt.Sprintf("%v", y)
	})
	assert.Empty(t, cmp.Diff(expected, actual, opt))
}
