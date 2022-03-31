// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package toenvconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/translator"

	"github.com/aws/amazon-cloudwatch-agent/translator/util"

	"os"

	commonconfig "github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/stretchr/testify/assert"
)

func ReadFromFile(filename string) string {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	str := string(data)
	return str
}

func checkIfTranslateSucceed(t *testing.T, jsonStr string, targetOs string, expectedEnvVars map[string]string) {
	var input map[string]interface{}
	translator.SetTargetPlatform(targetOs)
	err := json.Unmarshal([]byte(jsonStr), &input)
	if err == nil {
		envVarsBytes := ToEnvConfig(input)
		fmt.Println(string(envVarsBytes))
		var actualEnvVars = make(map[string]string)
		err := json.Unmarshal(envVarsBytes, &actualEnvVars)
		assert.NoError(t, err)
		assert.Equal(t, expectedEnvVars, actualEnvVars, "Expect to be equal")
	} else {
		fmt.Printf("Got error %v", err)
		t.Fail()
	}
}

func TestLogMetricOnly(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	os.Setenv(config.HOST_IP, "127.0.0.1")
	expectedEnvVars := map[string]string{}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/log_metric_only.json"), "linux", expectedEnvVars)
	os.Unsetenv(config.HOST_NAME)
	os.Unsetenv(config.HOST_IP)
}

func TestLogMetricAndLog(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	os.Setenv(config.HOST_IP, "127.0.0.1")
	expectedEnvVars := map[string]string{}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/log_metric_and_log.json"), "linux", expectedEnvVars)
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
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/complete_linux_config.json"), "linux", expectedEnvVars)
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/complete_windows_config.json"), "windows", expectedEnvVars)
}

func TestWindowsEventOnlyConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/windows_eventlog_only_config.json"), "windows", expectedEnvVars)
}

func TestStatsDConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/statsd_config.json"), "linux", expectedEnvVars)
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/statsd_config.json"), "windows", expectedEnvVars)
}

//Linux only for CollectD
func TestCollectDConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/collectd_config_linux.json"), "linux", expectedEnvVars)
}

func TestBasicConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/basic_config_linux.json"), "linux", expectedEnvVars)
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/basic_config_windows.json"), "windows", expectedEnvVars)
}

func TestStandardConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/standard_config_linux.json"), "linux", expectedEnvVars)
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/standard_config_windows.json"), "windows", expectedEnvVars)
}

func TestAdvancedConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/advanced_config_linux.json"), "linux", expectedEnvVars)
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/advanced_config_windows.json"), "windows", expectedEnvVars)
}

func TestLogOnlyConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/log_only_config_windows.json"), "windows", expectedEnvVars)
}

//test settings in commonconfig will override the ones in json config
func TestStandardConfigWithCommonConfig(t *testing.T) {
	resetContext()
	readCommonConifg()
	expectedEnvVars := map[string]string{
		"AWS_CA_BUNDLE": "/etc/test/ca_bundle.pem",
		"HTTPS_PROXY":   "https://127.0.0.1:3280",
		"HTTP_PROXY":    "http://127.0.0.1:3280",
		"NO_PROXY":      "254.1.1.1",
	}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/standard_config_linux.json"), "linux", expectedEnvVars)
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/standard_config_windows.json"), "windows", expectedEnvVars)
}

func TestCsmOnlyConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{
		"AWS_CSM_ENABLED": "TRUE",
	}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/csm_only_config.json"), "windows", expectedEnvVars)
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/csm_only_config.json"), "linux", expectedEnvVars)
}

func TestCsmServiceAddressesConfig(t *testing.T) {
	resetContext()
	expectedEnvVars := map[string]string{
		"AWS_CSM_ENABLED": "TRUE",
	}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/csm_service_addresses.json"), "windows", expectedEnvVars)
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/csm_service_addresses.json"), "linux", expectedEnvVars)
}

func TestECSNodeMetricConfig(t *testing.T) {
	resetContext()
	os.Setenv("RUN_IN_CONTAINER", "True")
	os.Setenv("HOST_NAME", "fake-host-name")
	os.Setenv("HOST_IP", "127.0.0.1")
	expectedEnvVars := map[string]string{}
	checkIfTranslateSucceed(t, ReadFromFile("../totomlconfig/sampleConfig/log_ecs_metric_only.json"), "linux", expectedEnvVars)
}

func readCommonConifg() {
	ctx := context.CurrentContext()
	conf := commonconfig.New()
	data, _ := ioutil.ReadFile("../totomlconfig/sampleConfig/commonConfigTest.toml")
	conf.Parse(bytes.NewReader(data))
	ctx.SetCredentials(conf.CredentialsMap())
	ctx.SetProxy(conf.ProxyMap())
	ctx.SetSSL(conf.SSLMap())
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
