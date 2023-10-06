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

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func startAgent(writer io.WriteCloser) error {
	if os.Getenv(config.RUN_IN_CONTAINER) == config.RUN_IN_CONTAINER_TRUE {
		// Use exec so PID 1 changes to agent from start-agent.
		execArgs := []string{
			paths.AgentBinaryPath, // when using syscall.Exec, must pass binary name as args[0]
			"-config", paths.TomlConfigPath,
			"-envconfig", paths.EnvConfigPath,
			"-otelconfig", paths.YamlConfigPath,
			"-pidfile", paths.AgentDir + "/var/amazon-cloudwatch-agent.pid",
		}
		if err := syscall.Exec(paths.AgentBinaryPath, execArgs, os.Environ()); err != nil {
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

	name, err := exec.LookPath(paths.AgentBinaryPath)
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
		paths.AgentBinaryPath,
		"-config", paths.TomlConfigPath,
		"-envconfig", paths.EnvConfigPath,
		"-otelconfig", paths.YamlConfigPath,
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
	ctx.SetInputJsonFilePath(paths.JsonConfigPath)
	ctx.SetInputJsonDirPath(paths.JsonDirPath)
	ctx.SetMultiConfig("remove")
	return cmdutil.GenerateMergedJsonConfigMap(ctx)
}
