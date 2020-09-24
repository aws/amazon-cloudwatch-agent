// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package totomlconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

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
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	str := string(data)
	return strings.ReplaceAll(str, "\r\n", "\n")
}

func checkIfTranslateSucceed(t *testing.T, jsonStr string, desiredTomlPath string, targetOs string) {
	agent.Global_Config = *new(agent.Agent)
	var input interface{}
	translator.SetTargetPlatform(targetOs)
	e := json.Unmarshal([]byte(jsonStr), &input)
	if e == nil {
		actualOutput := ToTomlConfig(input)
		desiredOutput := ReadFromFile(desiredTomlPath)
		assert.Equal(t, desiredOutput, actualOutput, "Expect to be equal")
	} else {
		fmt.Printf("Got error %v", e)
		t.Fail()
	}
}

func TestLogMetricOnly(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	os.Setenv(config.HOST_IP, "127.0.0.1")
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/log_metric_only.json"), "./sampleConfig/log_metric_only.conf", "linux")
	os.Unsetenv(config.HOST_NAME)
	os.Unsetenv(config.HOST_IP)
}

func TestLogMetricAndLog(t *testing.T) {
	resetContext()
	context.CurrentContext().SetRunInContainer(true)
	os.Setenv(config.HOST_NAME, "host_name_from_env")
	os.Setenv(config.HOST_IP, "127.0.0.1")
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/log_metric_and_log.json"), "./sampleConfig/log_metric_and_log.conf", "linux")
	os.Unsetenv(config.HOST_NAME)
	os.Unsetenv(config.HOST_IP)
}

func TestCompleteConfig(t *testing.T) {
	resetContext()
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/complete_linux_config.json"), "./sampleConfig/complete_linux_config.conf", "linux")
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/complete_windows_config.json"), "./sampleConfig/complete_windows_config.conf", "windows")
}

func TestWindowsEventOnlyConfig(t *testing.T) {
	resetContext()
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/windows_eventlog_only_config.json"), "./sampleConfig/windows_eventlog_only_config.conf", "windows")
}

func TestStatsDConfig(t *testing.T) {
	resetContext()
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/statsd_config.json"), "./sampleConfig/statsd_config_linux.conf", "linux")
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/statsd_config.json"), "./sampleConfig/statsd_config_windows.conf", "windows")
}

//Linux only for CollectD
func TestCollectDConfig(t *testing.T) {
	resetContext()
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/collectd_config_linux.json"), "./sampleConfig/collectd_config_linux.conf", "linux")
}

func TestBasicConfig(t *testing.T) {
	resetContext()
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/basic_config_linux.json"), "./sampleConfig/basic_config_linux.conf", "linux")
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/basic_config_windows.json"), "./sampleConfig/basic_config_windows.conf", "windows")
}

func TestStandardConfig(t *testing.T) {
	resetContext()
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/standard_config_linux.json"), "./sampleConfig/standard_config_linux.conf", "linux")
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/standard_config_windows.json"), "./sampleConfig/standard_config_windows.conf", "windows")
}

func TestAdvancedConfig(t *testing.T) {
	resetContext()
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/advanced_config_linux.json"), "./sampleConfig/advanced_config_linux.conf", "linux")
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/advanced_config_windows.json"), "./sampleConfig/advanced_config_windows.conf", "windows")
}

func TestLogOnlyConfig(t *testing.T) {
	resetContext()
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/log_only_config_windows.json"), "./sampleConfig/log_only_config_windows.conf", "windows")
}

func TestStandardConfigWithCommonConfig(t *testing.T) {
	resetContext()
	readCommonConifg()
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/standard_config_linux.json"), "./sampleConfig/standard_config_linux_with_common_config.conf", "linux")
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/standard_config_windows.json"), "./sampleConfig/standard_config_windows_with_common_config.conf", "windows")
}

func TestCsmOnlyConfig(t *testing.T) {
	resetContext()
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/csm_only_config.json"), "./sampleConfig/csm_only_config_windows.conf", "windows")
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/csm_only_config.json"), "./sampleConfig/csm_only_config_linux.conf", "linux")
}

func TestDeltaConfigLinux(t *testing.T) {
	resetContext()
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/delta_config_linux.json"), "./sampleConfig/delta_config_linux.conf", "linux")
}

func TestCsmServiceAdressesConfig(t *testing.T) {
	resetContext()
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/csm_service_addresses.json"), "./sampleConfig/csm_service_addresses_windows.conf", "windows")
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/csm_service_addresses.json"), "./sampleConfig/csm_service_addresses_linux.conf", "linux")
}

func TestECSNodeMetricConfig(t *testing.T) {
	resetContext()
	os.Setenv("RUN_IN_CONTAINER", "True")
	os.Setenv("HOST_NAME", "fake-host-name")
	os.Setenv("HOST_IP", "127.0.0.1")
	checkIfTranslateSucceed(t, ReadFromFile("./sampleConfig/log_ecs_metric_only.json"), "./sampleConfig/log_ecs_metric_only.conf", "linux")
}

func readCommonConifg() {
	ctx := context.CurrentContext()
	config := commonconfig.New()
	data, _ := ioutil.ReadFile("./sampleConfig/commonConfigTest.toml")
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
