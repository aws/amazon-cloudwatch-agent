// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tracesconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/data/config"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/xraydaemonmigration"
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
func (p *proc) Terminate() error {
	return nil
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
		cmdline:    []string{"xray", "-c", filepath.Join("testdata", "cfg.yaml"), "-b", addr + ":2000", "-t", addr + ":2000", "-a", "resourceTesting", "-n", "us-east-1", "-m", "23", "-r", "roleTest", "-p", "127.0.0.1:2000"},
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
	testutil.Type(inputChan, "2", "2000", "2000", "", "", "")
	jsonStruct, err = generateTracesConfiguration(ctx)
	assert.NotNil(t, jsonStruct) //changed to not nil due to returning defaults

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

func TestUserBuildsTracesConfig(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()
	testutil.Type(inputChan, "3000", "4000", "10", "20", "us-east-1", "This should be a number", "This should be a number", "", "", "", "", "")

	tracesConfig := &config.Traces{}

	whichUDPPort(tracesConfig)
	whichTCPPort(tracesConfig)
	chooseBufferSize(tracesConfig)
	chooseConcurrency(tracesConfig)
	chooseRegion(tracesConfig)

	//checking if variable of traces config are the values they supposed to be
	assert.Equal(t, addr+":3000", tracesConfig.TracesCollected.Xray.BindAddress)
	assert.Equal(t, addr+":4000", tracesConfig.TracesCollected.Xray.TcpProxy.BindAddress)
	assert.Equal(t, 10, tracesConfig.BufferSizeMB)
	assert.Equal(t, 20, tracesConfig.Concurrency)
	assert.Equal(t, "us-east-1", tracesConfig.RegionOverride)
	//wrong inputs
	chooseConcurrency(tracesConfig)
	chooseBufferSize(tracesConfig)
	assert.Equal(t, tracesConfig.Concurrency, 8)
	assert.Equal(t, tracesConfig.BufferSizeMB, 3)

	//blank answers (no input)
	whichUDPPort(tracesConfig)
	whichTCPPort(tracesConfig)
	chooseBufferSize(tracesConfig)
	chooseConcurrency(tracesConfig)
	chooseRegion(tracesConfig)
	assert.Equal(t, addr+":2000", tracesConfig.TracesCollected.Xray.BindAddress)
	assert.Equal(t, addr+":2000", tracesConfig.TracesCollected.Xray.TcpProxy.BindAddress)
	assert.Equal(t, 3, tracesConfig.BufferSizeMB)
	assert.Equal(t, 8, tracesConfig.Concurrency)
	assert.Equal(t, "", tracesConfig.RegionOverride)
}

func TestUpdateUserConfig(t *testing.T) {
	//Replicating user inputs
	inputs := []string{
		"1", addr + ":3000",
		"2", addr + ":4000",
		"3", "10",
		"4", "10",
		"5", "resourceArn",
		"6", "true",
		"7", "true",
		"8", "roleArn",
		"9", "endpointOverride",
		"10", "regionOverride",
		"11", "proxyOverride",
		"0",
	}
	inputChan := testutil.SetUpTestInputStream()
	testutil.Type(inputChan, inputs...)
	tracesConfig := &config.Traces{}

	//calling update function
	updateUserConfig(tracesConfig)

	//checking all tests
	assert.Equal(t, addr+":3000", tracesConfig.TracesCollected.Xray.BindAddress)
	assert.Equal(t, addr+":4000", tracesConfig.TracesCollected.Xray.TcpProxy.BindAddress)
	assert.Equal(t, 10, tracesConfig.BufferSizeMB)
	assert.Equal(t, 10, tracesConfig.Concurrency)
	assert.Equal(t, "resourceArn", tracesConfig.ResourceArn)
	assert.Equal(t, true, tracesConfig.LocalMode)
	assert.Equal(t, true, tracesConfig.Insecure)
	assert.Equal(t, "roleArn", tracesConfig.Credentials.RoleArn)
	assert.Equal(t, "endpointOverride", tracesConfig.EndpointOverride)
	assert.Equal(t, "regionOverride", tracesConfig.RegionOverride)
	assert.Equal(t, "proxyOverride", tracesConfig.ProxyOverride)

}
