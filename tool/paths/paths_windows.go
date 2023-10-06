// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package paths

import "os"

const (
	AgentDir = "\\Amazon\\AmazonCloudWatchAgent\\"
	JsonDir = "\\Configs"
	BinaryDir = "bin"
	TranslatorBinaryName = "config-translator.exe"
	AgentBinaryName = "amazon-cloudwatch-agent.exe"
	WizardBinaryName = "amazon-cloudwatch-agent-config-wizard.exe"
	AgentStartName = "amazon-cloudwatch-agent-ctl.ps1"
)

func init() {
	programFiles := os.Getenv("ProgramFiles")
	var programData string
	if _, ok := os.LookupEnv("ProgramData"); ok {
		programData = os.Getenv("ProgramData")
	} else {
		// Windows 2003
		programData = os.Getenv("ALLUSERSPROFILE") + "\\Application Data"
	}

	AgentRootDir := programFiles + paths.AgentDir
	AgentConfigDir := programData + paths.AgentDir

	JsonConfigPath = agentConfigDir + "\\" + JSON
	JsonDirPath = agentConfigDir + paths.JsonDir
	EnvConfigPath = agentConfigDir + "\\" + ENV
	TomlConfigPath = agentConfigDir + "\\" + TOML
	YamlConfigPath = agentConfigDir + "\\" + YAML
	CommonConfigPath = agentConfigDir + "\\" + COMMON_CONFIG
	AgentLogFilePath = agentConfigDir + "\\Logs\\" + AGENT_LOG_FILE
	TranslatorBinaryPath = agentRootDir + "\\" + paths.TranslatorBinaryName
	AgentBinaryPath = agentRootDir + "\\" + paths.AgentBinaryName
}