// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux || darwin
// +build linux darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

const (
	defaultConfigLocation = paths.AgentDir + "/" + paths.BinaryDir + "/config.json"
	pathToWizard          = paths.AgentDir + "/" + paths.BinaryDir + "/" + paths.WizardBinaryName
	pathToWizardDir       = paths.AgentDir + "/" + paths.BinaryDir
)

func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid: true,
	}
}
func FetchConfig() error {
	//Start cloudwatch with config built or user given
	cmd := execCommand("sudo", filepath.Join(paths.AgentDir, paths.BinaryDir, paths.AgentStartName), "-a", "fetch-config", "-m", "auto", "-s", "-c", "file:"+defaultConfigLocation) //file location needs to be dir of wizard + config.json
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	err := cmd.Start()
	return err
}

// appending traces config
func AppendConfig() error {
	cmd := execCommand("sudo", filepath.Join(paths.AgentDir, paths.BinaryDir, paths.AgentStartName), "-a", "append-config", "-m", "auto", "-s", "-c", "file:"+filepath.Join(pathToWizardDir, "config-traces.json")) //should change traces location to wizard dir
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	err := cmd.Start()
	return err
}

func StopXrayService() error {
	stopCmd := execCommand("sudo", "systemctl", "stop", "xray")
	err := stopCmd.Start()
	if err != nil {
		fmt.Println("There was an error: with stopping xray service", err)
		return err
	}

	fmt.Println("AWS Xray daemon service stopped")
	return nil
}

// check if amazon-cloudwatch-agent is active
func checkCWAStatus() bool {
	cmd := exec.Command("sudo", "systemctl", "is-active", "amazon-cloudwatch-agent") //returns either active or inactive
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "active"
}

// check if xray is active as a service
func checkXrayStatus() bool {
	cmd := exec.Command("sudo", "systemctl", "is-active", "xray") //returns either active or inactive
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "active"
}
