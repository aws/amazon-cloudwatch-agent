// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/xraydaemonmigration"
)

type proc struct {
	pid        int32
	name       string
	cmdline    []string
	cwd        string
	strCmdline string
}

func (p *proc) CmdlineSlice() ([]string, error) {
	return p.cmdline, nil
}
func (p *proc) Cwd() (string, error) {
	return p.cwd, nil
}
func (p *proc) Pid() int32 {
	return p.pid
}

func (p *proc) Terminate() error {
	return nil
}
func (p *proc) Cmdline() (string, error) {
	return p.strCmdline, nil
}

var mockProcesses = func() ([]xraydaemonmigration.Process, error) {

	var correctDaemonProcess = &proc{
		pid:     123,
		name:    "xray",
		cmdline: []string{"xray", "-c", filepath.Join("testdata", "cfg.yaml"), "-b", "127.0.0.1:2000", "-t", "127.0.0.1:2000", "-a", "resourceTesting", "-n", "us-east-1", "-m", "23", "-r", "roleTest", "-p", "127.0.0.1:2000"},
		cwd:     "",
	}

	var duplicateDaemonProcess = &proc{
		pid:     456,
		name:    "xray",
		cmdline: []string{"xray", "-c", filepath.Join("testdata", "cfg.yaml")},
		cwd:     "",
	}

	var randomProcess = &proc{
		pid:  789,
		name: "other",
	}

	var randomNoNameProcess = &proc{
		pid: 123123,
	}

	processes := []xraydaemonmigration.Process{correctDaemonProcess, duplicateDaemonProcess, randomProcess, randomNoNameProcess}
	return processes, nil
}

type mockCmd struct {
	startErr error
	waitErr  error
}

func (m *mockCmd) Start() error {
	return m.startErr
}
func (m *mockCmd) Wait() error {
	return m.waitErr
}
func (m *mockCmd) SetDir(dir string)                        {}
func (m *mockCmd) SetSysProcAttr(attr *syscall.SysProcAttr) {}
func (m *mockCmd) SetStdout(out *os.File)                   {}
func (m *mockCmd) SetStderr(err *os.File)                   {}

func TestTerminateXray(t *testing.T) {
	execCommand = func(_ string, _ ...string) CmdInterface {
		return &mockCmd{}
	}
	var xrayService = &proc{
		pid:     123,
		name:    "xray",
		cmdline: []string{filepath.Join("usr", "bin", "xray"), filepath.Join("var", "log", "xray", "xray.log")},
		cwd:     "",
	}
	err := TerminateXray(xrayService, func() bool { return false })
	assert.NoError(t, err)
}

func TestIsCWAOn(t *testing.T) {
	// Test when CWA is already active
	isOn := IsCWAOn(
		1*time.Second,
		func() bool { return true },
	)
	assert.True(t, isOn, "Expected CWA to be active immediately")

	// Test when CWA becomes active during the wait period
	isOn = IsCWAOn(
		30*time.Second,
		func() bool { return true },
	)
	assert.True(t, isOn, "Expected CWA to become active during wait")

	// Test when CWA never becomes active
	isOn = IsCWAOn(
		1*time.Second,
		func() bool { return false },
	)
	assert.False(t, isOn, "Expected CWA to never become active")
}

func TestConfigExists(t *testing.T) {
	configPath := filepath.Join("testdata", "config.json")
	_ = os.Remove(configPath)

	// Test when the config file does not exist
	assert.Equal(t, Fetch, configExists(configPath), "Expected 'Fetch' when config file does not exist")
	//Create Dir if it does not exist.
	err := os.MkdirAll("testdata", 0755)
	assert.NoError(t, err, "Failed to create directory")

	// Create a config file
	file, err := os.Create(configPath)
	assert.NoError(t, err, "Failed to create file")
	file.Close()

	// Test when the config file exists
	assert.Equal(t,
		Append, configExists(configPath), "Expected 'Append' when config file exists")

	err = os.Remove(configPath)
	assert.NoError(t, err, "Failed to remove file")
}

func TestRestartDaemon(t *testing.T) {
	mock := &mockCmd{}

	execCommand = func(_ string, _ ...string) CmdInterface {
		return mock
	}
	//successful command execution
	err := restartDaemon("testPath", []string{"testArg"}, "testCwd", false)
	assert.NoError(t, err)

	mock = &mockCmd{
		startErr: errors.New("start error"),
	}
	//Error when starting command
	err = restartDaemon("testPath", []string{"testArg"}, "testCwd", false)
	assert.Error(t, err)
	assert.Equal(t, "start error", err.Error())

	//Error when waiting for command to finish
	mock = &mockCmd{
		waitErr: errors.New("wait error"),
	}
	err = restartDaemon("testPath", []string{"testArg"}, "testCwd", false)
	assert.NoError(t, err)

	err = restartDaemon("testPath", []string{"testArg"}, "testCwd", true)
	assert.NoError(t, err)
}

func TestWithProcesses(t *testing.T) {
	xraydaemonmigration.GetProcesses = mockProcesses
	execCommand = func(_ string, _ ...string) CmdInterface {
		return &mockCmd{}
	}
	processes, err := xraydaemonmigration.FindAllDaemons()
	assert.NoError(t, err)
	greaterThan := len(processes) > 0
	assert.True(t, greaterThan)
	err = TerminateXray(processes[0], func() bool { return false })
	assert.NoError(t, err)

	//Fetch Config Case
	err = FetchConfig()
	assert.NoError(t, err)

	err = AppendConfig()
	assert.NoError(t, err)

	err = restartDaemon("testPath", []string{"testArg"}, "testCwd", false)
	assert.NoError(t, err)

}
