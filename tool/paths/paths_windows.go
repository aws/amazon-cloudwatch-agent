// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package paths

import "os"

const (
	AgentDir             = "\\Amazon\\AmazonCloudWatchAgent\\"
	JsonDir              = "\\Configs"
	BinaryDir            = "bin"
	TranslatorBinaryName = "config-translator.exe"
	AgentBinaryName      = "amazon-cloudwatch-agent.exe"
	WizardBinaryName     = "amazon-cloudwatch-agent-config-wizard.exe"
	AgentStartName       = "amazon-cloudwatch-agent-ctl.ps1"
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

	AgentRootDir := programFiles + AgentDir
	AgentConfigDir := programData + AgentDir

	JsonConfigPath = AgentConfigDir + "\\" + JSON
	JsonDirPath = AgentConfigDir + JsonDir
	EnvConfigPath = AgentConfigDir + "\\" + ENV
	TomlConfigPath = AgentConfigDir + "\\" + TOML
	YamlConfigPath = AgentConfigDir + "\\" + YAML
	CommonConfigPath = AgentConfigDir + "\\" + COMMON_CONFIG
	AgentLogFilePath = AgentConfigDir + "\\Logs\\" + AGENT_LOG_FILE
	TranslatorBinaryPath = AgentRootDir + "\\" + TranslatorBinaryName
	AgentBinaryPath = AgentRootDir + "\\" + AgentBinaryName
}
