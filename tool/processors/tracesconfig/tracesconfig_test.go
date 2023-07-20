// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tracesconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/testutil"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/xraydaemonmigration"
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
func (p *proc) Cmdline() (string, error) {
	return p.strcmdline, nil
}

var mockProcessesXrayService = func() ([]xraydaemonmigration.Process, error) {
	var xrayService = &proc{
		pid:     123,
		name:    "xray",
		cmdline: []string{filepath.Join("usr", "bin", "xray"), filepath.Join("var", "log", "xray", "xray.log")},
		cwd:     "",
	}
	processes := []xraydaemonmigration.Process{xrayService}
	return processes, nil
}

var mockProcesses = func() ([]xraydaemonmigration.Process, error) {

	var correctDaemonProcess = &proc{
		pid:        123,
		name:       "xray",
		cmdline:    []string{"xray", "-c", filepath.Join("testdata", "cfg.yaml"), "-b", "127.0.0.1:2000", "-t", "127.0.0.1:2000", "-a", "resourceTesting", "-n", "us-east-1", "-m", "23", "-r", "roleTest", "-p", "127.0.0.1:2000"},
		cwd:        "",
		strcmdline: "./xray -c ./cfg.yaml -b 127.0.0.1:2000 -t 127.0.0.1:2000 -a resourceTesting -n us-east-1 -m 23 -r roleTest -p 127.0.0.1:2000",
	}

	var duplicateDaemonProcess = &proc{
		pid:        456,
		name:       "xray",
		cmdline:    []string{"xray", "-c", filepath.Join("testdata", "cfg.yaml")},
		cwd:        "",
		strcmdline: "./xray -c ./cfg.yaml ",
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
var noProcesses = func() ([]xraydaemonmigration.Process, error) {
	return nil, nil
}
var incorrectInputProcess = func() ([]xraydaemonmigration.Process, error) {
	var incorrectInputProcess = &proc{
		pid:        456,
		name:       "xray",
		cmdline:    []string{"xray", "-c", filepath.Join("testdata", "IncorrecInput")},
		cwd:        "",
		strcmdline: "./xray -c ./cfg.yaml ",
	}
	processes := []xraydaemonmigration.Process{incorrectInputProcess}
	return processes, nil
}

func TestGenerateTracesConfiguration(t *testing.T) {
	//Command line test
	xraydaemonmigration.GetProcesses = mockProcesses
	inputChan := testutil.SetUpTestInputStream()
	testutil.Type(inputChan, "1", "2")
	ctx := &runtime.Context{TracesOnly: false}
	cmdlineConfigPath := filepath.Join("testdata", "configCmdline.json")
	cmdlineConfigFile, _ := os.ReadFile(cmdlineConfigPath)
	jsonStruct, err := generateTracesConfiguration(ctx)
	assert.NoError(t, err)
	jsonFile, err := json.Marshal(*jsonStruct)
	assert.NoError(t, err)
	assert.JSONEq(t, string(cmdlineConfigFile), string(jsonFile))
	ctx = &runtime.Context{TracesOnly: true}

	//Xray run as a servie
	xraydaemonmigration.GetProcesses = mockProcessesXrayService
	inputChan = testutil.SetUpTestInputStream()
	testutil.Type(inputChan, "2")
	jsonStruct, err = generateTracesConfiguration(ctx)
	assert.Nil(t, jsonStruct)

	//no processes test
	xraydaemonmigration.GetProcesses = noProcesses
	jsonStruct, err = generateTracesConfiguration(ctx)
	expectedDefaultConfigPath := filepath.Join("testdata", "defaultConfig.json")
	expectedDefaultConfigFile, err := os.ReadFile(expectedDefaultConfigPath)
	jsonFile, err = json.Marshal(*jsonStruct)
	assert.NoError(t, err)
	assert.JSONEq(t, string(expectedDefaultConfigFile), string(jsonFile))

	//Test for user with no Daemon process that inputs correct Daemon config path
	testutil.Type(inputChan, "1", filepath.Join("testdata", "cfg.yaml"))
	jsonStruct, err = generateTracesConfiguration(ctx)
	jsonFile, err = json.Marshal(*jsonStruct)
	assert.JSONEq(t, string(cmdlineConfigFile), string(jsonFile))

	//Test for user with no Daemon process that inputs incorrect Daemon config path
	testutil.Type(inputChan, "1", filepath.Join("testdata", "incorrect.yaml"))
	jsonStruct, err = generateTracesConfiguration(ctx)
	jsonFile, err = json.Marshal(*jsonStruct)
	assert.JSONEq(t, string(expectedDefaultConfigFile), string(jsonFile))

	//multiple proccess chose cmdline with no args
	xraydaemonmigration.GetProcesses = mockProcesses
	testutil.Type(inputChan, "3")
	jsonStruct, err = generateTracesConfiguration(ctx)
	jsonFile, err = json.MarshalIndent(*jsonStruct, "", "\t")
	noArgFilePath := filepath.Join("testdata", "noArg.json")
	noArgFile, err := os.ReadFile(noArgFilePath)
	assert.JSONEq(t, string(noArgFile), string(jsonFile))

	//incorrect config file path. FindConfigFile returns error. (Using default config)
	xraydaemonmigration.GetProcesses = incorrectInputProcess
	testutil.Type(inputChan, "2")
	jsonStruct, err = generateTracesConfiguration(ctx)
	jsonFile, err = json.MarshalIndent(*jsonStruct, "", "\t")
	assert.JSONEq(t, string(expectedDefaultConfigFile), string(jsonFile))
}
