// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package totomlconfig

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/aws/amazon-cloudwatch-agent/translator/totomlconfig/tomlConfigTemplate"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kr/pretty"

	"github.com/aws/amazon-cloudwatch-agent/translator"

	"github.com/aws/amazon-cloudwatch-agent/translator/util"

	"os"

	commonconfig "github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/stretchr/testify/assert"
)

func ReadFromFile(filename string) string {
	data, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	str := string(data)
	return strings.ReplaceAll(str, "\r\n", "\n")
}

func TestLogMetricOnly(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	os.Setenv(config.HOST_IP, "127.0.0.1")
	checkTomlTranslation(t, "./sampleConfig/log_metric_only.json", "./sampleConfig/log_metric_only.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/log_metric_only.json", "./sampleConfig/log_metric_only.conf", "darwin")
	os.Unsetenv(config.HOST_NAME)
	os.Unsetenv(config.HOST_IP)
}

func TestLogMetricOnPrem(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	os.Setenv(config.HOST_IP, "127.0.0.1")
	context.CurrentContext().SetMode(config.ModeOnPrem)
	checkTomlTranslation(t, "./sampleConfig/log_metric_only.json", "./sampleConfig/log_metric_only_on_prem.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/log_metric_only.json", "./sampleConfig/log_metric_only_on_prem.conf", "darwin")
	os.Unsetenv(config.HOST_NAME)
	os.Unsetenv(config.HOST_IP)
}

func TestLogMetricOnPremise(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	os.Setenv(config.HOST_IP, "127.0.0.1")
	context.CurrentContext().SetMode(config.ModeOnPremise)
	checkTomlTranslation(t, "./sampleConfig/log_metric_only.json", "./sampleConfig/log_metric_only_on_prem.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/log_metric_only.json", "./sampleConfig/log_metric_only_on_prem.conf", "darwin")
	os.Unsetenv(config.HOST_NAME)
	os.Unsetenv(config.HOST_IP)
}

func TestLogMetricAndLog(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	os.Setenv(config.HOST_IP, "127.0.0.1")
	checkTomlTranslation(t, "./sampleConfig/log_metric_and_log.json", "./sampleConfig/log_metric_and_log.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/log_metric_and_log.json", "./sampleConfig/log_metric_and_log.conf", "darwin")
	os.Unsetenv(config.HOST_NAME)
	os.Unsetenv(config.HOST_IP)
}

func TestLogMetricAndLogOnPrem(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	os.Setenv(config.HOST_IP, "127.0.0.1")
	context.CurrentContext().SetMode(config.ModeOnPrem)
	checkTomlTranslation(t, "./sampleConfig/log_metric_and_log.json", "./sampleConfig/log_metric_and_log_on_prem.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/log_metric_and_log.json", "./sampleConfig/log_metric_and_log_on_prem.conf", "darwin")
	os.Unsetenv(config.HOST_NAME)
	os.Unsetenv(config.HOST_IP)
}

func TestLogMetricAndLogOnPremise(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	os.Setenv(config.HOST_IP, "127.0.0.1")
	context.CurrentContext().SetMode(config.ModeOnPremise)
	checkTomlTranslation(t, "./sampleConfig/log_metric_and_log.json", "./sampleConfig/log_metric_and_log_on_prem.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/log_metric_and_log.json", "./sampleConfig/log_metric_and_log_on_prem.conf", "darwin")
	os.Unsetenv(config.HOST_NAME)
	os.Unsetenv(config.HOST_IP)
}

func TestCompleteConfig(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/complete_linux_config.json", "./sampleConfig/complete_linux_config.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/complete_darwin_config.json", "./sampleConfig/complete_darwin_config.conf", "darwin")
	checkTomlTranslation(t, "./sampleConfig/complete_windows_config.json", "./sampleConfig/complete_windows_config.conf", "windows")
}

func TestWindowsEventOnlyConfig(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/windows_eventlog_only_config.json", "./sampleConfig/windows_eventlog_only_config.conf", "windows")
}

func TestStatsDConfig(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/statsd_config.json", "./sampleConfig/statsd_config_linux.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/statsd_config.json", "./sampleConfig/statsd_config_linux.conf", "darwin")
	checkTomlTranslation(t, "./sampleConfig/statsd_config.json", "./sampleConfig/statsd_config_windows.conf", "windows")
}

// Linux only for CollectD
func TestCollectDConfig(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/collectd_config_linux.json", "./sampleConfig/collectd_config_linux.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/collectd_config_linux.json", "./sampleConfig/collectd_config_linux.conf", "darwin")
}

// prometheus
func TestPrometheusConfig(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	checkTomlTranslation(t, "./sampleConfig/prometheus_config_linux.json", "./sampleConfig/prometheus_config_linux.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/prometheus_config_windows.json", "./sampleConfig/prometheus_config_windows.conf", "windows")
	os.Unsetenv(config.HOST_NAME)
}

func TestBasicConfig(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/basic_config_linux.json", "./sampleConfig/basic_config_linux.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/basic_config_linux.json", "./sampleConfig/basic_config_linux.conf", "darwin")

	checkTomlTranslation(t, "./sampleConfig/basic_config_windows.json", "./sampleConfig/basic_config_windows.conf", "windows")
}

func TestStandardConfig(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/standard_config_linux.json", "./sampleConfig/standard_config_linux.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/standard_config_linux.json", "./sampleConfig/standard_config_linux.conf", "darwin")
	checkTomlTranslation(t, "./sampleConfig/standard_config_windows.json", "./sampleConfig/standard_config_windows.conf", "windows")
}

func TestAdvancedConfig(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/advanced_config_linux.json", "./sampleConfig/advanced_config_linux.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/advanced_config_linux.json", "./sampleConfig/advanced_config_linux.conf", "darwin")
	checkTomlTranslation(t, "./sampleConfig/advanced_config_windows.json", "./sampleConfig/advanced_config_windows.conf", "windows")
}

func TestDropOriginConfig(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/drop_origin_linux.json", "./sampleConfig/drop_origin_linux.conf", "linux")
}

func TestLogOnlyConfig(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/log_only_config_windows.json", "./sampleConfig/log_only_config_windows.conf", "windows")
}

func TestStandardConfigWithCommonConfig(t *testing.T) {
	resetContext()
	readCommonConfig()
	checkTomlTranslation(t, "./sampleConfig/standard_config_linux.json", "./sampleConfig/standard_config_linux_with_common_config.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/standard_config_linux.json", "./sampleConfig/standard_config_linux_with_common_config.conf", "darwin")
	checkTomlTranslation(t, "./sampleConfig/standard_config_windows.json", "./sampleConfig/standard_config_windows_with_common_config.conf", "windows")
}

func TestCsmOnlyConfig(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/csm_only_config.json", "./sampleConfig/csm_only_config_windows.conf", "windows")
	checkTomlTranslation(t, "./sampleConfig/csm_only_config.json", "./sampleConfig/csm_only_config_linux.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/csm_only_config.json", "./sampleConfig/csm_only_config_linux.conf", "darwin")
}

func TestDeltaConfigLinux(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/delta_config_linux.json", "./sampleConfig/delta_config_linux.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/delta_config_linux.json", "./sampleConfig/delta_config_linux.conf", "darwin")
}

func TestCsmServiceAddressesConfig(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/csm_service_addresses.json", "./sampleConfig/csm_service_addresses_windows.conf", "windows")
	checkTomlTranslation(t, "./sampleConfig/csm_service_addresses.json", "./sampleConfig/csm_service_addresses_linux.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/csm_service_addresses.json", "./sampleConfig/csm_service_addresses_linux.conf", "darwin")
}

func TestECSNodeMetricConfig(t *testing.T) {
	resetContext()
	os.Setenv("RUN_IN_CONTAINER", "True")
	os.Setenv("HOST_NAME", "fake-host-name")
	os.Setenv("HOST_IP", "127.0.0.1")
	checkTomlTranslation(t, "./sampleConfig/log_ecs_metric_only.json", "./sampleConfig/log_ecs_metric_only.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/log_ecs_metric_only.json", "./sampleConfig/log_ecs_metric_only.conf", "darwin")
	os.Unsetenv("RUN_IN_CONTAINER")
	os.Unsetenv("HOST_NAME")
	os.Unsetenv("HOST_IP")
}

func TestLogFilterConfig(t *testing.T) {
	resetContext()
	checkTomlTranslation(t, "./sampleConfig/log_filter.json", "./sampleConfig/log_filter.conf", "linux")
	checkTomlTranslation(t, "./sampleConfig/log_filter.json", "./sampleConfig/log_filter.conf", "darwin")
}

func TestTomlToTomlComparison(t *testing.T) {
	resetContext()
	var jsonFilePath = "./tomlConfigTemplate/agentToml.json"
	var input interface{}

	translator.SetTargetPlatform("linux")

	err := json.Unmarshal([]byte(ReadFromFile(jsonFilePath)), &input)
	assert.NoError(t, err)
	actualOutput := ToTomlConfig(input)
	checkIfIdenticalToml(t, "./tomlConfigTemplate/agentToml.conf", actualOutput)
}

func checkTomlTranslation(t *testing.T, jsonPath string, desiredTomlPath string, os string) {
	agent.Global_Config = *new(agent.Agent)
	translator.SetTargetPlatform(os)
	var input interface{}
	err := json.Unmarshal([]byte(ReadFromFile(jsonPath)), &input)
	assert.NoError(t, err)
	actualOutput := ToTomlConfig(input)
	log.Printf("output is %v", actualOutput)
	checkIfIdenticalToml(t, desiredTomlPath, actualOutput)
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
func checkIfIdenticalToml(t *testing.T, desiredTomlPath string, tomlStr string) {
	var expect tomlConfigTemplate.TomlConfig
	_, decodeError := toml.DecodeFile(desiredTomlPath, &expect)
	assert.NoError(t, decodeError)

	var actual tomlConfigTemplate.TomlConfig
	_, decodeError2 := toml.Decode(tomlStr, &actual)
	assert.NoError(t, decodeError2)
	// This less function sort the content of string slice in a alphabetical order so the
	// cmp.Equal method will compare the two struct with slices in them, regardless the elements within the slices
	opt := cmpopts.SortSlices(func(x, y interface{}) bool {
		return pretty.Sprint(x) < pretty.Sprint(y)
	})
	diff := cmp.Diff(expect, actual)
	log.Printf("D! Toml diff: %s", diff)
	assert.True(t, cmp.Equal(expect, actual, opt))
}
