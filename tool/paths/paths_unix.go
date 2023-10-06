// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux || darwin
// +build linux darwin

package paths

const (
	AgentDir = "/opt/aws/amazon-cloudwatch-agent"
	BinaryDir = "bin"
	JsonDir = "amazon-cloudwatch-agent.d"
	TranslatorBinaryName = "config-translator"
	AgentBinaryName = "amazon-cloudwatch-agent"
	WizardBinaryName = "amazon-cloudwatch-agent-config-wizard"
	AgentStartName = "amazon-cloudwatch-agent-ctl"
)

func init() {
	JsonConfigPath = AgentDir + "/etc/" + JSON
	JsonDirPath = AgentDir + "/etc/" + JsonDir
	EnvConfigPath = AgentDir + "/etc/" + ENV
	TomlConfigPath = AgentDir + "/etc/" + TOML
	CommonConfigPath = AgentDir + "/etc/" + COMMON_CONFIG
	YamlConfigPath = AgentDir + "/etc/" + YAML

	AgentLogFilePath = AgentDir + "/logs/" + AGENT_LOG_FILE

	TranslatorBinaryPath = AgentDir + "/bin/" + TranslatorBinaryName
	AgentBinaryPath = AgentDir + "/bin/" + AgentBinaryName
}