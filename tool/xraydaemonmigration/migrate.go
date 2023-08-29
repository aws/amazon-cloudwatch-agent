// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package xraydaemonmigration

import (
	_ "embed"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/process"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/tool/data/config"
)

type Process interface {
	Cwd() (string, error)
	CmdlineSlice() ([]string, error)
	Cmdline() (string, error)
	Terminate() error
}

// Daemon Yaml Configuration struct
type YamlConfig struct {
	TotalBufferSizeMB int    `yaml:"TotalBufferSizeMB"`
	Concurrency       int    `yaml:"Concurrency"`
	Region            string `yaml:"Region"`
	Socket            struct {
		UDPAddress string `yaml:"UDPAddress"`
		TCPAddress string `yaml:"TCPAddress"`
	} `yaml:"Socket"`
	LocalMode    bool   `yaml:"LocalMode"`
	ResourceARN  string `yaml:"ResourceARN"`
	RoleARN      string `yaml:"RoleARN"`
	ProxyAddress string `yaml:"ProxyAddress"`
	Endpoint     string `yaml:"Endpoint"`
	NoVerifySSL  bool   `yaml:"NoVerifySSL"`
}

// Setting flags for Daemon
func daemonFlagSet(yamlConfig *YamlConfig, process Process) error {
	newFlag := NewFlag("X-Ray Daemon")
	var configFilePath string
	newFlag.StringVarF(&yamlConfig.ResourceARN, "resource-arn", "a", yamlConfig.ResourceARN, "Amazon Resource Name (ARN) of the AWS resource running the daemon.")
	newFlag.BoolVarF(&yamlConfig.LocalMode, "local-mode", "o", yamlConfig.LocalMode, "Don't check for EC2 instance metadata.")
	newFlag.IntVarF(&yamlConfig.TotalBufferSizeMB, "buffer-memory", "m", yamlConfig.TotalBufferSizeMB, "Change the amount of memory in MB that buffers can use (minimum 3).")
	newFlag.StringVarF(&yamlConfig.Region, "region", "n", yamlConfig.Region, "Send segments to X-Ray service in a specific region.")
	newFlag.StringVarF(&yamlConfig.Socket.UDPAddress, "bind", "b", yamlConfig.Socket.UDPAddress, "Overrides default UDP address (127.0.0.1:2000).")
	newFlag.StringVarF(&yamlConfig.Socket.TCPAddress, "bind-tcp", "t", yamlConfig.Socket.TCPAddress, "Overrides default TCP address (127.0.0.1:2000).")
	newFlag.StringVarF(&yamlConfig.RoleARN, "role-arn", "r", yamlConfig.RoleARN, "Assume the specified IAM role to upload segments to a different account.")
	newFlag.StringVarF(&configFilePath, "config", "c", "", "Load a configuration file from the specified path.")
	newFlag.StringVarF(&yamlConfig.ProxyAddress, "proxy-address", "p", yamlConfig.ProxyAddress, "Proxy address through which to upload segments.")
	cmdline, err := process.CmdlineSlice()
	if err != nil {
		return err
	}

	if len(cmdline) != 0 {
		//if fails because of service we do not want user to see unnecessary logs.
		newFlag.fs.SetOutput(io.Discard)
		err = newFlag.fs.Parse(cmdline[1:])
		if err != nil {
			return err
		}
		//checking num of flag were passed through command line
		count := 0
		newFlag.fs.Visit(func(fl *flag.Flag) {
			count++
		})
		//xray is service or just use default config
		if count == 0 {
			return errors.New("no flags were passed")
		}
	}
	return nil
}

// Process Wrapper function
var GetProcesses = func() ([]Process, error) {
	processList, err := process.Processes()
	if err != nil {
		return nil, err
	}
	var xrayProcesses []Process
	for _, p := range processList {
		curName, err := p.Name()
		if err != nil {
			continue
		}
		if curName == "xray" {
			xrayProcesses = append(xrayProcesses, p)
		}
	}
	return xrayProcesses, nil
}

// Converting yaml Data to Json File. Process is needed to get command line arguments.
func ConvertYamlToJson(yamlData []byte, process Process) (*config.Traces, error) {

	//Defining JSON and YAML struct
	var jsonConfig config.Traces
	var yamlConfig YamlConfig
	//Add data to yaml and set flags
	err := yaml.Unmarshal(yamlData, &yamlConfig)
	if err != nil {
		return nil, err
	}
	if process != nil {
		err = daemonFlagSet(&yamlConfig, process)
	}

	if err != nil {
		return nil, err
	}

	//mapping YAML to JSON
	jsonConfig.TracesCollected.Xray.BindAddress = yamlConfig.Socket.UDPAddress
	jsonConfig.TracesCollected.Xray.TcpProxy.BindAddress = yamlConfig.Socket.TCPAddress
	jsonConfig.BufferSizeMB = yamlConfig.TotalBufferSizeMB
	jsonConfig.Concurrency = yamlConfig.Concurrency
	jsonConfig.RegionOverride = yamlConfig.Region
	jsonConfig.LocalMode = yamlConfig.LocalMode
	jsonConfig.ResourceArn = yamlConfig.ResourceARN
	if yamlConfig.RoleARN != "" {
		if jsonConfig.Credentials == nil {
			jsonConfig.Credentials = &struct {
				RoleArn string `json:"role_arn,omitempty"`
			}{}
		}

		jsonConfig.Credentials.RoleArn = yamlConfig.RoleARN
	}
	jsonConfig.ProxyOverride = yamlConfig.ProxyAddress
	jsonConfig.EndpointOverride = yamlConfig.Endpoint
	jsonConfig.Insecure = yamlConfig.NoVerifySSL

	//converts to JSON adding indentation to make output look nicer
	return &jsonConfig, nil
}

// Finds config file from cmdline
func FindConfigFile(process Process) (string, error) {

	argList, err := process.CmdlineSlice()
	if err != nil {
		return "", err
	}
	//got the path from command line (might not be exact path)
	path := GetPathFromArgs(argList)
	cwd, err := process.Cwd()
	if err != nil {
		return "", err
	}
	configFile := path
	if configFile == "" {
		return "", nil
	}
	//If the cwd in path, then that is the full path, otherwise add to config file path
	if filepath.IsAbs(path) {
		configFile = path
	} else {
		configFile = filepath.Join(cwd, path)
	}
	fileInfo, err := os.Stat(configFile)
	if err != nil {
		return "", err
	}
	if !fileInfo.IsDir() {
		return configFile, nil
	}
	return "", nil

}

// Finds all Daemons and returns their pid
func FindAllDaemons() ([]Process, error) {
	processes, err := GetProcesses()
	if err != nil {
		return nil, err
	}
	var returnList []Process
	for i := 0; i < len(processes); i++ {
		returnList = append(returnList, processes[i])
	}
	return returnList, nil
}

// Get the config file path from arguments
func GetPathFromArgs(argList []string) string {
	for i := 0; i < len(argList); i++ {
		arg := strings.Trim(strings.ToLower(argList[i]), "-")
		if arg == "c" || arg == "config" {
			if i+1 < len(argList) {
				return argList[i+1]
			} else {
				return ""
			}
		}
	}
	return ""

}
