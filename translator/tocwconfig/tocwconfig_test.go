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
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/toenvconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/totomlconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/totomlconfig/tomlConfigTemplate"
	"github.com/aws/amazon-cloudwatch-agent/translator/tocwconfig/toyamlconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/eksdetector"
)

const (
	prometheusFileNameToken = "prometheusFileName"
	ecsSdFileNamToken       = "ecsSdFileName"
)

//go:embed sampleConfig/prometheus_config.yaml
var prometheusConfig string

type testCase struct {
	filename        string
	targetPlatform  string
	expectedEnvVars map[string]string
	appendString    string
}

func TestBaseContainerInsightsConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetRunInContainer(true)
	context.CurrentContext().SetMode(config.ModeEC2)
	t.Setenv(config.HOST_NAME, "host_name_from_env")
	t.Setenv(config.HOST_IP, "127.0.0.1")
	t.Setenv(envconfig.AWS_CA_BUNDLE, "/etc/test/ca_bundle.pem")
	expectedEnvVars := map[string]string{
		"AWS_CA_BUNDLE": "/etc/test/ca_bundle.pem",
	}
	checkTranslation(t, "base_container_insights_config", "linux", expectedEnvVars, "")
	checkTranslation(t, "base_container_insights_config", "darwin", nil, "")
}

func TestGenericAppSignalsConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetRunInContainer(false)
	context.CurrentContext().SetMode(config.ModeOnPremise)
	t.Setenv(config.HOST_NAME, "host_name_from_env")
	t.Setenv(config.HOST_IP, "127.0.0.1")
	expectedEnvVars := map[string]string{"CWAGENT_LOG_LEVEL": "DEBUG"}
	checkTranslation(t, "base_appsignals_config", "linux", expectedEnvVars, "")
	checkTranslation(t, "base_appsignals_config", "windows", expectedEnvVars, "")
}

func TestGenericAppSignalsFallbackConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetRunInContainer(false)
	context.CurrentContext().SetMode(config.ModeOnPremise)
	t.Setenv(config.HOST_NAME, "host_name_from_env")
	t.Setenv(config.HOST_IP, "127.0.0.1")
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "base_appsignals_fallback_config", "linux", expectedEnvVars, "")
	checkTranslation(t, "base_appsignals_fallback_config", "windows", expectedEnvVars, "")
}

func TestAppSignalsAndEKSConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetRunInContainer(true)
	t.Setenv(config.HOST_NAME, "host_name_from_env")
	t.Setenv(config.HOST_IP, "127.0.0.1")
	t.Setenv(common.KubernetesEnvVar, "use_appsignals_eks_config")
	eksdetector.NewDetector = eksdetector.TestEKSDetector
	context.CurrentContext().SetMode(config.ModeEC2)
	context.CurrentContext().SetKubernetesMode(config.ModeEKS)

	expectedEnvVars := map[string]string{}
	checkTranslation(t, "appsignals_and_eks_config", "linux", expectedEnvVars, "")
	checkTranslation(t, "appsignals_and_eks_config", "windows", expectedEnvVars, "")
}

func TestAppSignalsFallbackAndEKSConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetRunInContainer(true)
	t.Setenv(config.HOST_NAME, "host_name_from_env")
	t.Setenv(config.HOST_IP, "127.0.0.1")
	t.Setenv(common.KubernetesEnvVar, "use_appsignals_eks_config")
	eksdetector.NewDetector = eksdetector.TestEKSDetector
	context.CurrentContext().SetMode(config.ModeEC2)
	context.CurrentContext().SetKubernetesMode(config.ModeEKS)

	expectedEnvVars := map[string]string{}
	checkTranslation(t, "appsignals_fallback_and_eks_config", "linux", expectedEnvVars, "")
	checkTranslation(t, "appsignals_fallback_and_eks_config", "windows", expectedEnvVars, "")
}

func TestAppSignalsFavorOverFallbackConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetRunInContainer(true)
	t.Setenv(config.HOST_NAME, "host_name_from_env")
	t.Setenv(config.HOST_IP, "127.0.0.1")
	t.Setenv(common.KubernetesEnvVar, "use_appsignals_eks_config")
	eksdetector.NewDetector = eksdetector.TestEKSDetector
	context.CurrentContext().SetMode(config.ModeEC2)
	context.CurrentContext().SetKubernetesMode(config.ModeEKS)

	expectedEnvVars := map[string]string{}
	checkTranslation(t, "appsignals_over_fallback_config", "linux", expectedEnvVars, "")
	checkTranslation(t, "appsignals_over_fallback_config", "windows", expectedEnvVars, "")
}

func TestAppSignalsAndNativeKubernetesConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetRunInContainer(true)
	t.Setenv(config.HOST_NAME, "host_name_from_env")
	t.Setenv(config.HOST_IP, "127.0.0.1")
	t.Setenv(common.KubernetesEnvVar, "use_appsignals_k8s_config")
	eksdetector.IsEKS = eksdetector.TestIsEKSCacheK8s
	context.CurrentContext().SetMode(config.ModeEC2)
	context.CurrentContext().SetKubernetesMode(config.ModeK8sEC2)

	expectedEnvVars := map[string]string{}
	checkTranslation(t, "appsignals_and_k8s_config", "linux", expectedEnvVars, "")
	checkTranslation(t, "appsignals_and_k8s_config", "windows", expectedEnvVars, "")
}

func TestEmfAndKubernetesConfig(t *testing.T) {
	resetContext(t)
	readCommonConfig(t, "./sampleConfig/commonConfig/withCredentials.toml")
	context.CurrentContext().SetRunInContainer(true)
	context.CurrentContext().SetMode(config.ModeOnPremise)
	t.Setenv(config.HOST_NAME, "host_name_from_env")
	t.Setenv(config.HOST_IP, "127.0.0.1")
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "emf_and_kubernetes_config", "linux", expectedEnvVars, "")
	checkTranslation(t, "emf_and_kubernetes_config", "darwin", nil, "")
}

func TestEmfAndKubernetesWithGpuConfig(t *testing.T) {
	resetContext(t)
	readCommonConfig(t, "./sampleConfig/commonConfig/withCredentials.toml")
	context.CurrentContext().SetRunInContainer(true)
	context.CurrentContext().SetMode(config.ModeOnPremise)
	t.Setenv(config.HOST_NAME, "host_name_from_env")
	t.Setenv(config.HOST_IP, "127.0.0.1")
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "emf_and_kubernetes_with_gpu_config", "linux", expectedEnvVars, "")
	checkTranslation(t, "emf_and_kubernetes_with_gpu_config", "darwin", nil, "")
}

func TestKubernetesModeOnPremiseConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetRunInContainer(true)
	context.CurrentContext().SetMode(config.ModeOnPremise)
	t.Setenv(config.HOST_NAME, "host_name_from_env")
	t.Setenv(config.HOST_IP, "127.0.0.1")
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "kubernetes_on_prem_config", "linux", expectedEnvVars, "")
}

func TestLogsAndKubernetesConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetRunInContainer(true)
	context.CurrentContext().SetMode(config.ModeEC2)
	t.Setenv(config.HOST_NAME, "host_name_from_env")
	t.Setenv(config.HOST_IP, "127.0.0.1")
	// for otel components and not our adapter components like
	// ec2 tagger processor we will have 0 for the imds number retry
	// in config instead of empty both become the value
	// both empty and 0 become 0 on converting of the yaml into a go struct
	// this is due to int defaulting to 0 in go
	t.Setenv(envconfig.IMDS_NUMBER_RETRY, "0")
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "logs_and_kubernetes_config", "linux", expectedEnvVars, "")
	checkTranslation(t, "logs_and_kubernetes_config", "darwin", nil, "")
}

func TestWindowsEventOnlyConfig(t *testing.T) {
	resetContext(t)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "windows_eventlog_only_config", "windows", expectedEnvVars, "")
}

func TestStatsDConfig(t *testing.T) {
	testCases := map[string]testCase{
		"linux": {
			filename:        "statsd_config",
			targetPlatform:  "linux",
			expectedEnvVars: map[string]string{},
			appendString:    "_linux",
		},
		"windows": {
			filename:        "statsd_config",
			targetPlatform:  "windows",
			expectedEnvVars: map[string]string{},
			appendString:    "_windows",
		},
		"darwin": {
			filename:        "statsd_config",
			targetPlatform:  "darwin",
			expectedEnvVars: nil,
			appendString:    "_linux",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			resetContext(t)
			context.CurrentContext().SetMode(config.ModeEC2)
			checkTranslation(t, testCase.filename, testCase.targetPlatform, testCase.expectedEnvVars, testCase.appendString)
		})
	}
}

// Linux only for CollectD
func TestCollectDConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "collectd_config_linux", "linux", expectedEnvVars, "")
	checkTranslation(t, "collectd_config_linux", "darwin", nil, "")
}

// prometheus
func TestPrometheusConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetRunInContainer(true)
	context.CurrentContext().SetMode(config.ModeEC2)
	t.Setenv(config.HOST_NAME, "host_name_from_env")
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
}

func TestBasicConfig(t *testing.T) {
	testCases := map[string]testCase{
		"linux": {
			filename:        "basic_config_linux",
			targetPlatform:  "linux",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
		"darwin": {
			filename:        "basic_config_linux",
			targetPlatform:  "darwin",
			expectedEnvVars: nil,
			appendString:    "",
		},
		"windows": {
			filename:        "basic_config_windows",
			targetPlatform:  "windows",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			resetContext(t)
			context.CurrentContext().SetMode(config.ModeEC2)
			checkTranslation(t, testCase.filename, testCase.targetPlatform, testCase.expectedEnvVars, testCase.appendString)
		})
	}
}

func TestInvalidInputConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "invalid_input_linux", "linux", expectedEnvVars, "")
}

func TestStandardConfig(t *testing.T) {
	// the way our config translator works is int(0) leaves an empty in the yaml
	// this will default to 0 on contrib side since int default is 0 for golang
	testCases := map[string]testCase{
		"linux": {
			filename:        "standard_config_linux",
			targetPlatform:  "linux",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
		"darwin": {
			filename:        "standard_config_linux",
			targetPlatform:  "darwin",
			expectedEnvVars: nil,
			appendString:    "",
		},
		"windows": {
			filename:        "standard_config_windows",
			targetPlatform:  "windows",
			expectedEnvVars: nil,
			appendString:    "",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			resetContext(t)
			context.CurrentContext().SetMode(config.ModeEC2)
			t.Setenv(envconfig.IMDS_NUMBER_RETRY, "0")
			checkTranslation(t, testCase.filename, testCase.targetPlatform, testCase.expectedEnvVars, testCase.appendString)
		})
	}
}

func TestAdvancedConfig(t *testing.T) {
	testCases := map[string]testCase{
		"linux": {
			filename:        "advanced_config_linux",
			targetPlatform:  "linux",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
		"darwin": {
			filename:        "advanced_config_darwin",
			targetPlatform:  "darwin",
			expectedEnvVars: nil,
			appendString:    "",
		},
		"windows": {
			filename:        "advanced_config_windows",
			targetPlatform:  "windows",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			resetContext(t)
			context.CurrentContext().SetMode(config.ModeEC2)
			checkTranslation(t, testCase.filename, testCase.targetPlatform, testCase.expectedEnvVars, testCase.appendString)
		})
	}
}

func TestLogOnlyConfig(t *testing.T) {
	resetContext(t)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "log_only_config_windows", "windows", expectedEnvVars, "")
}

func TestSkipLogTimestampConfig(t *testing.T) {
	testCases := map[string]testCase{
		"default_linux": {
			filename:        "skip_log_timestamp_default",
			targetPlatform:  "linux",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
		"default_darwin": {
			filename:        "skip_log_timestamp_default",
			targetPlatform:  "darwin",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
		"default_windows": {
			filename:        "skip_log_timestamp_default_windows",
			targetPlatform:  "windows",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
		"set_linux": {
			filename:        "skip_log_timestamp",
			targetPlatform:  "linux",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
		"set_darwin": {
			filename:        "skip_log_timestamp",
			targetPlatform:  "darwin",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
		"set_windows": {
			filename:        "skip_log_timestamp_windows",
			targetPlatform:  "windows",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
		"no_skip_linux": {
			filename:        "no_skip_log_timestamp",
			targetPlatform:  "linux",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
		"no_skip_darwin": {
			filename:        "no_skip_log_timestamp",
			targetPlatform:  "darwin",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
		"no_skip_windows": {
			filename:        "no_skip_log_timestamp_windows",
			targetPlatform:  "windows",
			expectedEnvVars: map[string]string{},
			appendString:    "",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			resetContext(t)
			checkTranslation(t, testCase.filename, testCase.targetPlatform, testCase.expectedEnvVars, testCase.appendString)
		})
	}
}

func TestDoNotSkipLogDefaultTimestampConfig(t *testing.T) {
	resetContext(t)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "log_only_config_windows", "windows", expectedEnvVars, "")
}

func TestTraceConfig(t *testing.T) {
	testCases := map[string]testCase{
		"linux": {
			filename:        "trace_config",
			targetPlatform:  "linux",
			expectedEnvVars: map[string]string{},
			appendString:    "_linux",
		},
		"darwin": {
			filename:        "trace_config",
			targetPlatform:  "darwin",
			expectedEnvVars: map[string]string{},
			appendString:    "_linux",
		},
		"windows": {
			filename:        "trace_config",
			targetPlatform:  "windows",
			expectedEnvVars: map[string]string{},
			appendString:    "_windows",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			resetContext(t)
			context.CurrentContext().SetMode(config.ModeEC2)
			readCommonConfig(t, "./sampleConfig/commonConfig/withCredentials.toml")
			checkTranslation(t, testCase.filename, testCase.targetPlatform, testCase.expectedEnvVars, testCase.appendString)
		})
	}
}

func TestConfigWithEnvironmentVariables(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "config_with_env", "linux", expectedEnvVars, "")
}

func TestStandardConfigWithCommonConfig(t *testing.T) {
	testCases := map[string]testCase{
		"linux": {
			filename:       "standard_config_linux",
			targetPlatform: "linux",
			expectedEnvVars: map[string]string{
				"AWS_CA_BUNDLE": "/etc/test/ca_bundle.pem",
				"HTTPS_PROXY":   "https://127.0.0.1:3280",
				"HTTP_PROXY":    "http://127.0.0.1:3280",
				"NO_PROXY":      "254.1.1.1",
			},
			appendString: "_with_common_config",
		},
		"darwin": {
			filename:        "standard_config_linux",
			targetPlatform:  "darwin",
			expectedEnvVars: nil,
			appendString:    "_with_common_config",
		},
		"windows": {
			filename:       "standard_config_windows",
			targetPlatform: "windows",
			expectedEnvVars: map[string]string{
				"AWS_CA_BUNDLE": "/etc/test/ca_bundle.pem",
				"HTTPS_PROXY":   "https://127.0.0.1:3280",
				"HTTP_PROXY":    "http://127.0.0.1:3280",
				"NO_PROXY":      "254.1.1.1",
			},
			appendString: "_with_common_config",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			resetContext(t)
			context.CurrentContext().SetMode(config.ModeEC2)
			readCommonConfig(t, "./sampleConfig/commonConfig/withCredentialsProxySsl.toml")
			checkTranslation(t, testCase.filename, testCase.targetPlatform, testCase.expectedEnvVars, testCase.appendString)
		})
	}
}

func TestDeltaNetConfigLinux(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "delta_net_config_linux", "linux", expectedEnvVars, "")
	checkTranslation(t, "delta_net_config_linux", "darwin", nil, "")
}

func TestECSNodeMetricConfig(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetRunInContainer(true)
	context.CurrentContext().SetMode(config.ModeEC2)
	t.Setenv("RUN_IN_CONTAINER", "True")
	t.Setenv("HOST_NAME", "fake-host-name")
	t.Setenv("HOST_IP", "127.0.0.1")
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "log_ecs_metric_only", "linux", expectedEnvVars, "")
	checkTranslation(t, "log_ecs_metric_only", "darwin", nil, "")
}

func TestLogFilterConfig(t *testing.T) {
	resetContext(t)
	checkTranslation(t, "log_filter", "linux", nil, "")
	checkTranslation(t, "log_filter", "darwin", nil, "")
}

func TestIgnoreInvalidAppendDimensions(t *testing.T) {
	resetContext(t)
	context.CurrentContext().SetMode(config.ModeEC2)
	expectedEnvVars := map[string]string{}
	checkTranslation(t, "ignore_append_dimensions", "linux", expectedEnvVars, "")
}

func TestTomlToTomlComparison(t *testing.T) {
	resetContext(t)
	var jsonFilePath = "./totomlconfig/tomlConfigTemplate/agentToml.json"
	var input interface{}
	x := os.Getenv("HOST_NAME")
	require.Equal(t, "", x)
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

func readCommonConfig(t *testing.T, commonConfigFilePath string) {
	ctx := context.CurrentContext()
	cfg := commonconfig.New()
	data, _ := os.ReadFile(commonConfigFilePath)
	require.NoError(t, cfg.Parse(bytes.NewReader(data)))
	ctx.SetCredentials(cfg.CredentialsMap())
	ctx.SetProxy(cfg.ProxyMap())
	ctx.SetSSL(cfg.SSLMap())
	util.LoadImdsRetries(cfg.IMDS)
}

func resetContext(t *testing.T) {
	t.Setenv(envconfig.IMDS_NUMBER_RETRY, strconv.Itoa(retryer.DefaultImdsRetries))
	util.DetectRegion = func(string, map[string]string) (string, string) {
		return "us-west-2", "ACJ"
	}
	util.DetectCredentialsPath = func() string {
		return "fake-path"
	}
	context.ResetContext()

	t.Setenv("ProgramData", "c:\\ProgramData")
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
		// assert.Equal(t, expected, actual) // this is useful for debugging differences between the YAML

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
