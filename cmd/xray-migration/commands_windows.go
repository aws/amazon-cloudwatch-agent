// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

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
	defaultConfigLocation = paths.AgentDir + "\\" + paths.BinaryDir + "\\config.json"
	pathToWizard          = paths.AgentDir + "\\" + paths.BinaryDir + "\\" + paths.WizardBinaryName
	pathToWizardDir       = paths.AgentDir + "\\" + paths.BinaryDir
)

func newSysProcAttr() *syscall.SysProcAttr {
	return nil
}
func FetchConfig() error {
	//Start cloudwatch with config built or user given
	cmd := execCommand("&", filepath.Join("C:\\Program Files\\Amazon\\AmazonCloudWatchAgent", paths.AgentStartName), "-a", "fetch-config", "-m", "auto", "-s", "-c", "file:"+defaultConfigLocation) //file location needs to be dir of wizard + config.json
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	err := cmd.Start()
	return err

}

// appending traces config
func AppendConfig() error {
	//Not sure if this the right permissions.
	cmd := execCommand("&", filepath.Join("C:\\Program Files\\Amazon\\AmazonCloudWatchAgent", paths.AgentStartName), "-a", "append-config", "-m", "auto", "-s", "-c", "file:"+filepath.Join(pathToWizardDir, "configFileTraces.json")) //file location needs to be dir of wizard + config.json
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	err := cmd.Start()
	return err
}

func StopXrayService() error {
	stopCmd := exec.Command("net", "stop", "AWSXRayDaemon")
	err := stopCmd.Run()
	if err != nil {
		fmt.Println("There was an error: with stopping xray service", err)
		return err
	}
	fmt.Println("AWS Xray daemon service stopped")
	return nil
}

// check if amazon-cloudwatch-agent is active
func checkCWAStatus() bool {
	cmd := exec.Command("sc", "query", "amazon-cloudwatch-agent")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "RUNNING")
}
func checkXrayStatus() bool {
	cmd := exec.Command("sc", "query", "xray")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "RUNNING")
}
