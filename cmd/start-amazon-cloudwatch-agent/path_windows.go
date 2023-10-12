// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package main

import (
	"fmt"
	"io"
	"log"
	"os/exec"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

func startAgent(writer io.WriteCloser) error {
	if err := writer.Close(); err != nil {
		log.Printf("E! Cannot close the log file, ERROR is %v \n", err)
		return err
	}

	cmd := exec.Command(
		paths.AgentBinaryPath,
		"-config", paths.TomlConfigPath,
		"-envconfig", paths.EnvConfigPath,
		"-otelconfig", paths.YamlConfigPath,
	)
	stdoutStderr, err := cmd.CombinedOutput()
	// log file is closed, so use fmt here
	fmt.Printf("%s \n", stdoutStderr)
	return err
}
