// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

func startAgent(writer io.WriteCloser) error {
	if err := writer.Close(); err != nil {
		log.Printf("E! Cannot close the log file, ERROR is %v \n", err)
		return err
	}

	cmd := exec.Command(
		agentBinaryPath,
		"-config", tomlConfigPath,
		"-envconfig", envConfigPath,
		"-otelconfig", yamlConfigPath,
	)
	stdoutStderr, err := cmd.CombinedOutput()
	// log file is closed, so use fmt here
	fmt.Printf("%s \n", stdoutStderr)
	return err
}

func init() {
	programFiles := os.Getenv("ProgramFiles")
	var programData string
	if _, ok := os.LookupEnv("ProgramData"); ok {
		programData = os.Getenv("ProgramData")
	} else {
		// Windows 2003
		programData = os.Getenv("ALLUSERSPROFILE") + "\\Application Data"
	}

	agentRootDir := programFiles + paths.AgentDir
	agentConfigDir := programData + paths.AgentDir

	jsonConfigPath = agentConfigDir + "\\" + JSON
	jsonDirPath = agentConfigDir + paths.JsonDir
	envConfigPath = agentConfigDir + "\\" + ENV
	tomlConfigPath = agentConfigDir + "\\" + TOML
	yamlConfigPath = agentConfigDir + "\\" + YAML

	commonConfigPath = agentConfigDir + "\\" + COMMON_CONFIG

	agentLogFilePath = agentConfigDir + "\\Logs\\" + AGENT_LOG_FILE

	translatorBinaryPath = agentRootDir + "\\" + paths.TranslatorBinaryName
	agentBinaryPath = agentRootDir + "\\" + paths.AgentBinaryName
}
