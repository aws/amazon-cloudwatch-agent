// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package paths

const (
	COMMON_CONFIG  = "common-config.toml"
	JSON           = "amazon-cloudwatch-agent.json"
	TOML           = "amazon-cloudwatch-agent.toml"
	YAML           = "amazon-cloudwatch-agent.yaml"
	ENV            = "env-config.json"
	AGENT_LOG_FILE = "amazon-cloudwatch-agent.log"
	//TODO this CONFIG_DIR_IN_CONTAINER should change to something indicate dir, keep it for now to avoid break testing
	CONFIG_DIR_IN_CONTAINER = "/etc/cwagentconfig"
)

var (
	JsonConfigPath       string
	JsonDirPath          string
	EnvConfigPath        string
	TomlConfigPath       string
	CommonConfigPath     string
	YamlConfigPath       string
	AgentLogFilePath     string
	TranslatorBinaryPath string
	AgentBinaryPath      string
)
