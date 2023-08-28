// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package xraydaemonmigration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

type proc struct {
	pid        int32
	name       string
	cmdline    []string
	cwd        string
	strcmdline string
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
func (p *proc) Cmdline() (string, error) {
	return p.strcmdline, nil
}

func (p *proc) Terminate() error {
	return nil
}

var mockProcesses = func() ([]Process, error) {

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

	processes := []Process{correctDaemonProcess, duplicateDaemonProcess, randomProcess, randomNoNameProcess}
	return processes, nil
}

var mockProcessesNone = func() ([]Process, error) {
	return nil, nil
}

var _ Process = (*proc)(nil)

func TestAllDaemonFunction(t *testing.T) {
	GetProcesses = mockProcesses
	result, err := FindAllDaemons()
	assert.NoError(t, err)
	assert.Equal(t, 4, len(result))
	GetProcesses = mockProcessesNone
	result, err = FindAllDaemons()
	assert.NoError(t, err)
	assert.Equal(t, []Process([]Process(nil)), result)
}

func TestGetPathFromArgs(t *testing.T) {
	testPaths := [][]string{
		{"xray", "-c", filepath.Join("testdata", "cfg.yaml")},
		{"xray", "-c", "cfg.yaml"},
		{"xray", "-c"},
		nil,
	}

	expected := []string{
		filepath.Join("testdata", "cfg.yaml"),
		"cfg.yaml",
		"",
		"",
	}

	var result []string
	for i := 0; i < len(testPaths); i++ {
		tempString := GetPathFromArgs(testPaths[i])
		result = append(result, tempString)
	}
	assert.Equal(t, expected, result)
}

func TestFindConfigFile(t *testing.T) {
	var correctDaemonProcess = &proc{
		pid:     123,
		name:    "xray",
		cmdline: []string{"xray", "-c", filepath.Join("testdata", "cfg.yaml"), "-b", "127.0.0.1:2000", "-t", "127.0.0.1:2000", "-a", "resourceTesting", "-n", "us-east-1", "-m", "23", "-r", "roleTest", "-p", "127.0.0.1:2000"},
	}
	result, err := FindConfigFile(correctDaemonProcess)

	assert.Equal(t, filepath.Join("testdata", "cfg.yaml"), result)
	assert.NoError(t, err)

}

func TestCovertYamlToJson(t *testing.T) {
	GetProcesses = mockProcesses
	wd, _ := os.Getwd()

	configFilePath := filepath.Join("testdata", "cfg.yaml")
	incorrectYaml := filepath.Join("testdata", "wrongCfg.yaml")
	yamlFile, err := os.ReadFile(incorrectYaml)
	assert.NoError(t, err)
	var duplicateDaemonProcess = &proc{
		pid:     456,
		name:    "xray",
		cmdline: []string{"xray", "-c", filepath.Join("testdata", "cfg.yaml")},
		cwd:     filepath.Join(wd),
	}
	jsonStruct, err := ConvertYamlToJson(yamlFile, duplicateDaemonProcess)
	assert.NotNil(t, err)

	exptectedFilePath := filepath.Join("testdata", "actualConfig.json")
	yamlFile, err = os.ReadFile(configFilePath)
	assert.NoError(t, err)
	var correctDaemonProcess = &proc{
		pid:     123,
		name:    "xray",
		cmdline: []string{"xray", "-c", filepath.Join("testdata", "cfg.yaml"), "-b", "127.0.0.1:2000", "-t", "127.0.0.1:2000", "-a", "resourceTesting", "-n", "us-east-1", "-m", "23", "-r", "roleTest", "-p", "127.0.0.1:2000"},
		cwd:     filepath.Join(wd),
	}
	jsonStruct, err = ConvertYamlToJson(yamlFile, correctDaemonProcess)
	assert.NoError(t, err)

	exptectedFile, err := os.ReadFile(exptectedFilePath)
	assert.NoError(t, err)

	jsonFile, err := json.MarshalIndent(jsonStruct, "", "\t")
	assert.NoError(t, err)
	assert.JSONEq(t, string(exptectedFile), string(jsonFile))
}
