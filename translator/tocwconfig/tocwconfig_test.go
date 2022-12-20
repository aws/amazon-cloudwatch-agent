// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tocwconfig

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/cfg/commonconfig"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/cmdutil"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/config"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/context"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toenvconfig"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/totomlconfig"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/totomlconfig/tomlConfigTemplate"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/tocwconfig/toyamlconfig"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/agent"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util"
)

const (
	prometheusFileNameToken = "prometheusFileName"
	ecsSdFileNamToken       = "ecsSdFileName"
)

//go:embed sampleConfig/prometheus_config.yaml
var prometheusConfig string

func TestLogMetricOnly(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	os.Setenv(config.HOST_IP, "127.0.0.1")
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "log_metric_only", "linux", expectedEnvVars, "")
	checkTranslation(t, "log_metric_only", "darwin", nil, "")
	os.Unsetenv(config.HOST_NAME)
	os.Unsetenv(config.HOST_IP)
}

func TestLogMetricAndLog(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	os.Setenv(config.HOST_IP, "127.0.0.1")
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "log_metric_and_log", "linux", expectedEnvVars, "")
	checkTranslation(t, "log_metric_and_log", "darwin", nil, "")
	os.Unsetenv(config.HOST_NAME)
	os.Unsetenv(config.HOST_IP)
}

func TestCompleteConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{
		"CWAGENT_USER_AGENT": "CUSTOM USER AGENT VALUE",
		"CWAGENT_LOG_LEVEL":  "DEBUG",
		"AWS_SDK_LOG_LEVEL":  "LogDebug",
	}
	checkTranslation(t, "complete_linux_config", "linux", expectedEnvVars, "")
	checkTranslation(t, "complete_darwin_config", "darwin", nil, "")
	checkTranslation(t, "complete_windows_config", "windows", expectedEnvVars, "")
}

func TestWindowsEventOnlyConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "windows_eventlog_only_config", "windows", expectedEnvVars, "")
}

func TestStatsDConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "statsd_config", "linux", expectedEnvVars, "_linux")
	checkTranslation(t, "statsd_config", "windows", expectedEnvVars, "_windows")
	checkTranslation(t, "statsd_config", "darwin", nil, "_linux")
}

// Linux only for CollectD
func TestCollectDConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "collectd_config_linux", "linux", expectedEnvVars, "")
	checkTranslation(t, "collectd_config_linux", "darwin", nil, "")
}

// prometheus
func TestPrometheusConfig(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	temp := t.TempDir()
	prometheusConfigFileName := filepath.Join(temp, "prometheus.yaml")
	ecsSdFileName := filepath.Join(temp, "ecs_sd_results.yaml")
	expectedEnvVars := map[string]string{}
	tokenReplacements := map[string]string{
		prometheusFileNameToken: strings.ReplaceAll(prometheusConfigFileName, "\\", "\\\\"),
		ecsSdFileNamToken:       strings.ReplaceAll(ecsSdFileName, "\\", "\\\\"),
	}
	// Load prometheus config and replace ecs sd results file name token with temp file name
	prometheusConfig = strings.ReplaceAll(prometheusConfig, "{"+ecsSdFileNamToken+"}", ecsSdFileName)
	// Write the modified prometheus config to temp prometheus config file
	err := os.WriteFile(prometheusConfigFileName, []byte(prometheusConfig), os.ModePerm)
	require.NoError(t, err)
	// In the following checks, we first load the json and replace tokens with the temp files
	// Additionally, before comparing with actual, we again replace tokens with temp files in the expected toml & yaml
	checkTranslation(t, "prometheus_config_linux", "linux", expectedEnvVars, "", tokenReplacements)
	checkTranslation(t, "prometheus_config_windows", "windows", nil, "", tokenReplacements)
	os.Unsetenv(config.HOST_NAME)
}

func TestBasicConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "basic_config_linux", "linux", expectedEnvVars, "")
	checkTranslation(t, "basic_config_linux", "darwin", nil, "")
	checkTranslation(t, "basic_config_windows", "windows", expectedEnvVars, "")
}

func TestStandardConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "standard_config_linux", "linux", expectedEnvVars, "")
	checkTranslation(t, "standard_config_linux", "darwin", nil, "")
	checkTranslation(t, "standard_config_windows", "windows", nil, "")
}

func TestAdvancedConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "advanced_config_linux", "linux", expectedEnvVars, "")
	checkTranslation(t, "advanced_config_linux", "darwin", nil, "")
	checkTranslation(t, "advanced_config_windows", "windows", expectedEnvVars, "")
}

func TestDropOriginConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "drop_origin_linux", "linux", expectedEnvVars, "")
}

func TestLogOnlyConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "log_only_config_windows", "windows", expectedEnvVars, "")
}

func TestStandardConfigWithCommonConfig(t *testing.T) {
	resetContext()
	readCommonConfig()
	expectedEnvVars := map[string]string{
		"AWS_CA_BUNDLE": "/etc/test/ca_bundle.pem",
		"HTTPS_PROXY":   "https://127.0.0.1:3280",
		"HTTP_PROXY":    "http://127.0.0.1:3280",
		"NO_PROXY":      "254.1.1.1",
	}
	checkTranslation(t, "standard_config_linux", "linux", expectedEnvVars, "_with_common_config")
	checkTranslation(t, "standard_config_linux", "darwin", nil, "_with_common_config")
	checkTranslation(t, "standard_config_windows", "windows", expectedEnvVars, "_with_common_config")
}

func TestCsmOnlyConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{
		"AWS_CSM_ENABLED": "TRUE",
	}
	checkTranslation(t, "csm_only_config", "windows", expectedEnvVars, "_windows")
	checkTranslation(t, "csm_only_config", "linux", expectedEnvVars, "_linux")
	checkTranslation(t, "csm_only_config", "darwin", nil, "_linux")
}

func TestDeltaConfigLinux(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "delta_config_linux", "linux", expectedEnvVars, "")
	checkTranslation(t, "delta_config_linux", "darwin", nil, "")
}

func TestCsmServiceAddressesConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{
		"AWS_CSM_ENABLED": "TRUE",
	}
	checkTranslation(t, "csm_service_addresses", "windows", expectedEnvVars, "_windows")
	checkTranslation(t, "csm_service_addresses", "linux", expectedEnvVars, "_linux")
	checkTranslation(t, "csm_service_addresses", "darwin", nil, "_linux")
}

func TestECSNodeMetricConfig(t *testing.T) {
	resetContext()
	os.Setenv("RUN_IN_CONTAINER", "True")
	os.Setenv("HOST_NAME", "fake-host-name")
	os.Setenv("HOST_IP", "127.0.0.1")
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "log_ecs_metric_only", "linux", expectedEnvVars, "")
	checkTranslation(t, "log_ecs_metric_only", "darwin", nil, "")
	os.Unsetenv("RUN_IN_CONTAINER")
	os.Unsetenv("HOST_NAME")
	os.Unsetenv("HOST_IP")
}

func TestLogFilterConfig(t *testing.T) {
	resetContext()
	checkTranslation(t, "log_filter", "linux", nil, "")
	checkTranslation(t, "log_filter", "darwin", nil, "")
}

func TestTomlToTomlComparison(t *testing.T) {
	resetContext()
	var jsonFilePath = "./totomlconfig/tomlConfigTemplate/agentToml.json"
	var input interface{}

	translator.SetTargetPlatform("linux")
	content, err := os.ReadFile(jsonFilePath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(content, &input))
	verifyToTomlTranslation(t, input, "./totomlconfig/tomlConfigTemplate/agentToml.conf", map[string]string{})
}

func checkTranslation(t *testing.T, fileName string, targetPlatform string, expectedEnvVars map[string]string, appendString string, tokenReplacements ...map[string]string) {
	jsonFilePath := fmt.Sprintf("./sampleConfig/%v.json", fileName)
	tomlFilePath := fmt.Sprintf("./sampleConfig/%v%v.conf", fileName, appendString)
	yamlFilePath := fmt.Sprintf("./sampleConfig/%v%v.yaml", fileName, appendString)
	checkTranslationForPaths(t, jsonFilePath, tomlFilePath, yamlFilePath, targetPlatform, tokenReplacements...)
	if expectedEnvVars != nil {
		content, err := os.ReadFile(jsonFilePath)
		require.NoError(t, err)
		checkIfEnvTranslateSucceed(t, string(content), targetPlatform, expectedEnvVars)
	}
}

func checkTranslationForPaths(t *testing.T, jsonFilePath string, expectedTomlFilePath string, expectedYamlFilePath string, targetPlatform string, tokenReplacements ...map[string]string) {
	agent.Global_Config = *new(agent.Agent)
	translator.SetTargetPlatform(targetPlatform)
	var input interface{}
	blob, err := os.ReadFile(jsonFilePath)
	content := replaceTokens(blob, tokenReplacements...)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(content), &input))
	verifyToTomlTranslation(t, input, expectedTomlFilePath, tokenReplacements...)
	verifyToYamlTranslation(t, input, expectedYamlFilePath, tokenReplacements...)
}

func readCommonConfig() {
	ctx := context.CurrentContext()
	config := commonconfig.New()
	data, _ := os.ReadFile("./sampleConfig/commonConfigTest.toml")
	config.Parse(bytes.NewReader(data))
	ctx.SetCredentials(config.CredentialsMap())
	ctx.SetProxy(config.ProxyMap())
	ctx.SetSSL(config.SSLMap())
}

func resetContext() {
	util.DetectRegion = func(string, map[string]string) string {
		return "us-west-2"
	}
	util.DetectCredentialsPath = func() string {
		return "fake-path"
	}
	context.ResetContext()

	os.Setenv("ProgramData", "c:\\ProgramData")
}

// toml files in the given path will be parsed into the config toml struct and be compared as struct
func verifyToTomlTranslation(t *testing.T, input interface{}, desiredTomlPath string, tokenReplacements ...map[string]string) {
	t.Helper()
	tomlConfig, err := cmdutil.TranslateJsonMapToTomlConfig(input)
	assert.NoError(t, err)

	tomlStr := totomlconfig.ToTomlConfig(tomlConfig)
	var expect tomlConfigTemplate.TomlConfig
	blob, err := os.ReadFile(desiredTomlPath)
	assert.NoError(t, err)
	content := replaceTokens(blob, tokenReplacements...)
	_, decodeError := toml.Decode(content, &expect)
	assert.NoError(t, decodeError)

	var actual tomlConfigTemplate.TomlConfig
	_, decodeError2 := toml.Decode(tomlStr, &actual)
	assert.NoError(t, decodeError2)
	// This less function sort the content of string slice in alphabetical order so the
	// cmp.Equal method will compare the two struct with slices in them, regardless the elements within the slices
	opt := cmpopts.SortSlices(func(x, y interface{}) bool {
		return pretty.Sprint(x) < pretty.Sprint(y)
	})
	assert.True(t, cmp.Equal(expect, actual, opt), "D! TOML diff: %s", cmp.Diff(expect, actual))
}

func verifyToYamlTranslation(t *testing.T, input interface{}, expectedYamlFilePath string, tokenReplacements ...map[string]string) {
	t.Helper()

	// if the file doesn't exist, then that means it isn't supported yet, so the
	// YAML translation should fail.
	if _, err := os.Stat(expectedYamlFilePath); errors.Is(err, fs.ErrNotExist) {
		yamlConfig, err := cmdutil.TranslateJsonMapToYamlConfig(input)
		require.Error(t, err)
		require.Nil(t, yamlConfig)
	} else {
		var expected interface{}
		bs, err := os.ReadFile(expectedYamlFilePath)
		require.NoError(t, err)
		content := replaceTokens(bs, tokenReplacements...)
		content = strings.ReplaceAll(content, "\\\\", "\\")
		require.NoError(t, yaml.Unmarshal([]byte(content), &expected))

		var actual interface{}
		yamlConfig, err := cmdutil.TranslateJsonMapToYamlConfig(input)
		require.NoError(t, err)
		yamlStr := toyamlconfig.ToYamlConfig(yamlConfig)
		require.NoError(t, yaml.Unmarshal([]byte(yamlStr), &actual))

		opt := cmpopts.SortSlices(func(x, y interface{}) bool {
			return pretty.Sprint(x) < pretty.Sprint(y)
		})
		require.True(t, cmp.Equal(expected, actual, opt), "D! YAML diff: %s", cmp.Diff(expected, actual))
	}
}

func replaceTokens(base []byte, tokenReplacements ...map[string]string) string {
	content := string(base)
	for _, replacements := range tokenReplacements {
		for token, replacement := range replacements {
			content = strings.ReplaceAll(content, strings.Join([]string{"{", token, "}"}, ""), replacement)
		}
	}
	return content
}

func checkIfEnvTranslateSucceed(t *testing.T, jsonStr string, targetOs string, expectedEnvVars map[string]string) {
	var input map[string]interface{}
	translator.SetTargetPlatform(targetOs)
	err := json.Unmarshal([]byte(jsonStr), &input)
	if err == nil {
		envVarsBytes := toenvconfig.ToEnvConfig(input)
		var actualEnvVars = make(map[string]string)
		err := json.Unmarshal(envVarsBytes, &actualEnvVars)
		assert.NoError(t, err)
		assert.Equal(t, expectedEnvVars, actualEnvVars, "Expect to be equal")
	} else {
		t.Logf("Got error %v", err)
		t.Fail()
	}
}
