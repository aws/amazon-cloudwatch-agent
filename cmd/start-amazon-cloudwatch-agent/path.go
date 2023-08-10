// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux || darwin
// +build linux darwin

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/paths"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/cmdutil"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/config"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/context"
)

func startAgent(writer io.WriteCloser) error {
	if os.Getenv(config.RUN_IN_CONTAINER) == config.RUN_IN_CONTAINER_TRUE {
		// Use exec so PID 1 changes to agent from start-agent.
		execArgs := []string{
			agentBinaryPath, // when using syscall.Exec, must pass binary name as args[0]
			"-config", tomlConfigPath,
			"-envconfig", envConfigPath,
			"-otelconfig", yamlConfigPath,
			"-pidfile", paths.AgentDir + "/var/amazon-cloudwatch-agent.pid",
		}
		if err := syscall.Exec(agentBinaryPath, execArgs, os.Environ()); err != nil {
			return fmt.Errorf("error exec as agent binary: %w", err)
		}
		// We should never reach this line but the compiler doesn't know...
		return nil
	}

	mergedJsonConfigMap, err := generateMergedJsonConfigMap()
	if err != nil {
		log.Printf("E! Failed to generate merged json config: %v ", err)
		return err
	}

	_, err = cmdutil.ChangeUser(mergedJsonConfigMap)
	if err != nil {
		log.Printf("E! Failed to ChangeUser: %v ", err)
		return err
	}

	name, err := exec.LookPath(agentBinaryPath)
	if err != nil {
		log.Printf("E! Failed to lookpath: %v ", err)
		return err
	}

	if err := writer.Close(); err != nil {
		log.Printf("E! Cannot close the log file, ERROR is %v ", err)
		return err
	}

	// linux command has pid passed while windows does not
	agentCmd := []string{
		agentBinaryPath,
		"-config", tomlConfigPath,
		"-envconfig", envConfigPath,
		"-otelconfig", yamlConfigPath,
		"-pidfile", paths.AgentDir + "/var/amazon-cloudwatch-agent.pid",
	}
	if err = syscall.Exec(name, agentCmd, os.Environ()); err != nil {
		// log file is closed, so use fmt here
		fmt.Printf("E! Exec failed: %v \n", err)
		return err
	}

	return nil
}

func generateMergedJsonConfigMap() (map[string]interface{}, error) {
	ctx := context.CurrentContext()
	setCTXOS(ctx)
	ctx.SetInputJsonFilePath(jsonConfigPath)
	ctx.SetInputJsonDirPath(jsonDirPath)
	ctx.SetMultiConfig("remove")
	return cmdutil.GenerateMergedJsonConfigMap(ctx)
}

func init() {
	jsonConfigPath = paths.AgentDir + "/etc/" + JSON
	jsonDirPath = paths.AgentDir + "/etc/" + paths.JsonDir
	envConfigPath = paths.AgentDir + "/etc/" + ENV
	tomlConfigPath = paths.AgentDir + "/etc/" + TOML
	commonConfigPath = paths.AgentDir + "/etc/" + COMMON_CONFIG
	yamlConfigPath = paths.AgentDir + "/etc/" + YAML

	agentLogFilePath = paths.AgentDir + "/logs/" + AGENT_LOG_FILE

	translatorBinaryPath = paths.AgentDir + "/bin/" + paths.TranslatorBinaryName
	agentBinaryPath = paths.AgentDir + "/bin/" + paths.AgentBinaryName
}
