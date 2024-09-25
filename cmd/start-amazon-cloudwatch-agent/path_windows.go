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

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/config"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

func startAgent(writer io.WriteCloser) error {
	if !envconfig.IsRunningInContainer() {
		if err := writer.Close(); err != nil {
			log.Printf("E! Cannot close the log file, ERROR is %v \n", err)
			return err
		}
		execArgs := []string{
			"-config", paths.TomlConfigPath,
			"-envconfig", paths.EnvConfigPath,
		}
		execArgs = append(execArgs, config.GetOTELConfigArgs(paths.ConfigDirPath)...)
		cmd := exec.Command(paths.AgentBinaryPath, execArgs...)
		stdoutStderr, err := cmd.CombinedOutput()
		// log file is closed, so use fmt here
		fmt.Printf("%s \n", stdoutStderr)
		return err
	} else {
		execArgs := []string{
			"-config", paths.TomlConfigPath,
			"-envconfig", paths.EnvConfigPath,
		}
		execArgs = append(execArgs, config.GetOTELConfigArgs(paths.CONFIG_DIR_IN_CONTAINER)...)
		execArgs = append(execArgs, "-console", "true")
		cmd := exec.Command(paths.AgentBinaryPath, execArgs...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		fmt.Printf("%s \n", err)
		return err
	}

}
