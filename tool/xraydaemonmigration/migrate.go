// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package xraydaemonmigration

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/process"
	"gopkg.in/yaml.v3"
)

type Process interface {
	Cwd() (string, error)
	CmdlineSlice() ([]string, error)
}

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
	RoleARN      string `yaml:"string"`
	ProxyAddress string `yaml:"ProxyAddress"`
	Endpoint     string `yaml:"Endpoint"`
	NoVerifySSL  bool   `yaml:"NoVerifySSL"`
}
type JsonConfig struct {
	Traces struct {
		TracesCollected struct {
			Xray struct {
				BindAddress string `json:"bind_address"`
				TcpProxy    struct {
					BindAddress string `json:"bind_address"`
				} `json:"tcp_proxy"`
			} `json:"xray"`
		} `json:"traces_collected"`
		Concurrency  int    `json:"concurrency"`
		BufferSizeMB int    `json:"buffer_size_mb"`
		ResourceArn  string `json:"resource_arn"`
		LocalMode    bool   `json:"local_mode"` //local
		Insecure     bool   `json:"insecure"`   //noverifyssl
		Credentials  struct {
			RoleArn string `json:"role_arn"`
		} `json:"credentials"`
		EndpointOverride string `json:"endpoint_override"` //endpoint
		RegionOverride   string `json:"region_override"`   //region
		ProxyOverride    string `json:"proxy_override"`
	} `json:"traces"`
}

func daemonFlagSet(yamlConfig YamlConfig, process Process) (YamlConfig, error) {
	var configFilePath string
	flag := NewFlag("X-Ray Daemon")

	flag.StringVarF(&yamlConfig.ResourceARN, "resource-arn", "a", yamlConfig.ResourceARN, "Amazon Resource Name (ARN) of the AWS resource running the daemon.")
	flag.BoolVarF(&yamlConfig.LocalMode, "local-mode", "o", yamlConfig.LocalMode, "Don't check for EC2 instance metadata.")
	flag.IntVarF(&yamlConfig.TotalBufferSizeMB, "buffer-memory", "m", yamlConfig.TotalBufferSizeMB, "Change the amount of memory in MB that buffers can use (minimum 3).")
	flag.StringVarF(&yamlConfig.Region, "region", "n", yamlConfig.Region, "Send segments to X-Ray service in a specific region.")
	flag.StringVarF(&yamlConfig.Socket.UDPAddress, "bind", "b", yamlConfig.Socket.UDPAddress, "Overrides default UDP address (127.0.0.1:2000).")
	flag.StringVarF(&yamlConfig.Socket.TCPAddress, "bind-tcp", "t", yamlConfig.Socket.TCPAddress, "Overrides default TCP address (127.0.0.1:2000).")
	flag.StringVarF(&yamlConfig.RoleARN, "role-arn", "r", yamlConfig.RoleARN, "Assume the specified IAM role to upload segments to a different account.")
	flag.StringVarF(&configFilePath, "config", "c", "", "Load a configuration file from the specified path.")
	flag.StringVarF(&yamlConfig.ProxyAddress, "proxy-address", "p", yamlConfig.ProxyAddress, "Proxy address through which to upload segments.")
	cmdline, err := process.CmdlineSlice()
	if err != nil {
		return yamlConfig, err
	}
	if len(cmdline) != 0 {
		flag.fs.Parse(cmdline[1:])
	}
	return yamlConfig, nil
}

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

// Converting yaml Data to Json File. Pid is needed to get command line arguments of the process (if Daemon is running as a process and not a service).
func ConvertYamlToJson(yamlData []byte, process Process) ([]byte, error) {

	var jsonConfig JsonConfig
	var yamlConfig YamlConfig
	err := yaml.Unmarshal(yamlData, &yamlConfig)
	yamlConfig, err = daemonFlagSet(yamlConfig, process)
	if err != nil {
		return nil, err
	}
	jsonConfig.Traces.TracesCollected.Xray.BindAddress = yamlConfig.Socket.UDPAddress
	jsonConfig.Traces.TracesCollected.Xray.TcpProxy.BindAddress = yamlConfig.Socket.TCPAddress
	jsonConfig.Traces.BufferSizeMB = yamlConfig.TotalBufferSizeMB
	jsonConfig.Traces.Concurrency = yamlConfig.Concurrency
	jsonConfig.Traces.RegionOverride = yamlConfig.Region
	jsonConfig.Traces.LocalMode = yamlConfig.LocalMode
	jsonConfig.Traces.ResourceArn = yamlConfig.ResourceARN
	jsonConfig.Traces.Credentials.RoleArn = yamlConfig.RoleARN
	jsonConfig.Traces.ProxyOverride = yamlConfig.ProxyAddress
	jsonConfig.Traces.EndpointOverride = yamlConfig.Endpoint
	jsonConfig.Traces.Insecure = yamlConfig.NoVerifySSL
	//converts to JSON adding indentation to make output look nicer
	jsonData, _ := json.MarshalIndent(jsonConfig, "", "\t")

	return jsonData, nil
}

func FindAllPotentialConfigFiles() ([]string, error) {
	var allPotentialConfigFiles []string
	//loop through all Daemons
	processes, err := GetProcesses()
	if err != nil {
		return nil, err
	}
	if len(processes) == 0 || processes == nil {
		return nil, nil
	}
	for i := 0; i < len(processes); i++ {
		argList, err := processes[i].CmdlineSlice()
		if err != nil || len(argList) == 0 {
			continue
		}
		//got the path from command line (might not be exact path)
		path := GetPathFromArgs(argList)
		cwd, err := processes[i].Cwd()
		if err != nil {
			return nil, err
		}

		configFile := path
		//If the cwd in path, then that is the full path, otherwise add to config file path
		if filepath.IsAbs(path) {
			configFile = path
		} else {
			configFile = filepath.Join(cwd, path)
		}

		allPotentialConfigFiles = append(allPotentialConfigFiles, configFile)

	}
	if len(allPotentialConfigFiles) == 0 {
		return nil, nil
	}
	//printing the config file the user has access too.
	return allPotentialConfigFiles, nil
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

// get the config file path from arguments
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
