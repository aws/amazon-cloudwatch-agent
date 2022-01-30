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
)

const (
	AGENT_DIR_WINDOWS = "\\Amazon\\AmazonCloudWatchAgent\\"

	JSON_DIR_WINDOWS = "\\Configs"

	TRANSLATOR_BINARY_WINDOWS = "config-translator.exe"
	AGENT_BINARY_WINDOWS      = "amazon-cloudwatch-agent.exe"
)

func startAgent(writer io.WriteCloser) error {
	if err := writer.Close(); err != nil {
		log.Printf("E! Cannot close the log file, ERROR is %v \n", err)
		return err
	}

	cmd := exec.Command(agentBinaryPath, "-config", tomlConfigPath, "-envconfig", envConfigPath)
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

	agentRootDir := programFiles + AGENT_DIR_WINDOWS
	agentConfigDir := programData + AGENT_DIR_WINDOWS

	jsonConfigPath = agentConfigDir + "\\" + JSON
	jsonDirPath = agentConfigDir + JSON_DIR_WINDOWS
	envConfigPath = agentConfigDir + "\\" + ENV
	tomlConfigPath = agentConfigDir + "\\" + TOML

	commonConfigPath = agentConfigDir + "\\" + COMMON_CONFIG

	agentLogFilePath = agentConfigDir + "\\Logs\\" + AGENT_LOG_FILE

	translatorBinaryPath = agentRootDir + "\\" + TRANSLATOR_BINARY_WINDOWS
	agentBinaryPath = agentRootDir + "\\" + AGENT_BINARY_WINDOWS
}
