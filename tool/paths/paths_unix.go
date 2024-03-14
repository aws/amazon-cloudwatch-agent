// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux || darwin
// +build linux darwin

package paths

import "path/filepath"

const (
	AgentDir             = "/opt/aws/amazon-cloudwatch-agent"
	BinaryDir            = "bin"
	JsonDir              = "amazon-cloudwatch-agent.d"
	TranslatorBinaryName = "config-translator"
	AgentBinaryName      = "amazon-cloudwatch-agent"
	WizardBinaryName     = "amazon-cloudwatch-agent-config-wizard"
	AgentStartName       = "amazon-cloudwatch-agent-ctl"
	//TODO this CONFIG_DIR_IN_CONTAINER should change to something indicate dir, keep it for now to avoid break testing
	CONFIG_DIR_IN_CONTAINER = "/etc/cwagentconfig"
)

func init() {
	JsonConfigPath = filepath.Join(AgentDir, "etc", JSON)
	JsonDirPath = filepath.Join(AgentDir, "etc", JsonDir)
	EnvConfigPath = filepath.Join(AgentDir, "etc", ENV)
	TomlConfigPath = filepath.Join(AgentDir, "etc", TOML)
	CommonConfigPath = filepath.Join(AgentDir, "etc", COMMON_CONFIG)
	YamlConfigPath = filepath.Join(AgentDir, "etc", YAML)
	AgentLogFilePath = filepath.Join(AgentDir, "logs", AGENT_LOG_FILE)
	TranslatorBinaryPath = filepath.Join(AgentDir, "bin", TranslatorBinaryName)
	AgentBinaryPath = filepath.Join(AgentDir, "bin", AgentBinaryName)
}
