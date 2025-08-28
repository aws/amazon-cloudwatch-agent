// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package paths

const (
	COMMON_CONFIG          = "common-config.toml"
	JSON                   = "amazon-cloudwatch-agent.json"
	TOML                   = "amazon-cloudwatch-agent.toml"
	YAML                   = "amazon-cloudwatch-agent.yaml"
	ENV                    = "env-config.json"
	AGENT_LOG_FILE         = "amazon-cloudwatch-agent.log"
	AGENT_LOG_FILE_INPUTS  = "amazon-cloudwatch-agent-inputs.log"
	AGENT_LOG_FILE_OUTPUTS = "amazon-cloudwatch-agent-outputs.log"
	JMXJarName             = "opentelemetry-jmx-metrics.jar"
)

var (
	JsonConfigPath          string
	ConfigDirPath           string
	EnvConfigPath           string
	TomlConfigPath          string
	CommonConfigPath        string
	YamlConfigPath          string
	AgentLogFilePath        string
	AgentLogFileInputsPath  string
	AgentLogFileOutputsPath string
	TranslatorBinaryPath    string
	AgentBinaryPath         string
	JMXJarPath              string
)
