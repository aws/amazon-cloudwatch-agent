// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tracesconfig

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/data/config"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/serialization"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
	"github.com/aws/amazon-cloudwatch-agent/tool/xraydaemonmigration"
)

var Processor processors.Processor = &processor{}

type processor struct{}

const addr = "127.0.0.1"

func (p *processor) Process(ctx *runtime.Context, cfg *data.Config) {
	//skip if linux or windows noninteractive migration
	if ctx.WindowsNonInteractiveMigration {
		return
	}

	if !ctx.TracesOnly {
		yes := util.Yes("Do you want the CloudWatch agent to also retrieve X-ray traces?")
		if !yes {
			return
		}
	}
	newTraces, err := generateTracesConfiguration(ctx)
	if err != nil || newTraces == nil {
		return
	}
	cfg.TracesConfig = newTraces
	//user can review and update their current configurations
	if cfg.TracesConfig != nil && !ctx.NonInteractiveXrayMigration {
		cfg.TracesConfig = updateUserConfig(cfg.TracesConfig)
	}
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	return serialization.Processor
}

//go:embed configtraces.json
var DefaultTracesConfigFile []byte

const (
	anyExistingDaemonConfiguration = "Do you have an existing X-Ray Daemon configuration file to import for migration?"
	filePathXrayConfigQuestion     = "What is the file path for the existing X-Ray Daemon configuration file?"
)

func userBuildsTracesConfig(tracesConfig *config.Traces) *config.Traces {

	json.Unmarshal(DefaultTracesConfigFile, &tracesConfig)
	whichUDPPort(tracesConfig)
	whichTCPPort(tracesConfig)
	chooseBufferSize(tracesConfig)
	chooseConcurrency(tracesConfig)
	chooseRegion(tracesConfig)
	return tracesConfig
}

func whichUDPPort(tracesConfig *config.Traces) {
	answer := util.AskWithDefault("Which UDP port do you want XRay daemon to listen to?", "2000")
	num, err := strconv.Atoi(answer)
	if err != nil || num < 0 {
		tracesConfig.TracesCollected.Xray.BindAddress = addr + ":2000"
	}
	newAddr := fmt.Sprintf("%s:%s", addr, answer)
	tracesConfig.TracesCollected.Xray.BindAddress = newAddr

}
func whichTCPPort(tracesConfig *config.Traces) {
	answer := util.AskWithDefault("Which TCP port do you want XRay daemon to listen to?", "2000")
	num, err := strconv.Atoi(answer)
	if err != nil || num < 0 {
		tracesConfig.TracesCollected.Xray.BindAddress = addr + ":2000"
	}
	newAddr := fmt.Sprintf("%s:%s", addr, answer)
	tracesConfig.TracesCollected.Xray.TcpProxy.BindAddress = newAddr

}

func chooseBufferSize(tracesConfig *config.Traces) {
	answer := util.AskWithDefault("Enter Total Buffer Size in MB (minimum 3)", "3")
	bufferSize, err := strconv.Atoi(answer)
	if err != nil || bufferSize < 3 {
		fmt.Println("Buffer size set to 3 because input smaller than 3 or not a number")
		bufferSize = 3
	}
	tracesConfig.BufferSizeMB = bufferSize
}

func chooseConcurrency(tracesConfig *config.Traces) {
	answer := util.AskWithDefault("Enter the maximum number of concurrent calls to AWS X-Ray to upload segment documents: ", "8")
	concurrency, err := strconv.Atoi(answer)
	if err != nil || concurrency < 0 {
		fmt.Println("Concurrency set to default value of 8 because input smaller than 0 or not a number")
		concurrency = 8
	}
	tracesConfig.Concurrency = concurrency
}

func chooseRegion(tracesConfig *config.Traces) {
	answer := util.Ask("Enter the AWS Region to send segments to AWS X-Ray service (Optional)")
	tracesConfig.RegionOverride = answer

}

// made to print traces config correctly
type TracesWrapper struct {
	Traces config.Traces `json:"traces"`
}

func updateUserConfig(tracesConfig *config.Traces) *config.Traces {

	fieldOptions := generateFieldOptions()
	for {
		jsonData := TracesWrapper{
			Traces: *tracesConfig,
		}
		fmt.Println("Current Traces Configurations:")
		jsonByte, _ := json.MarshalIndent(jsonData, "", "\t")
		fmt.Println(string(jsonByte))
		fmt.Println("Enter a number of the field you would like to update (or 0 to exit)")
		for i := 0; i < len(fieldOptions); i++ {
			fmt.Println(fieldOptions[i])
		}
		answer := util.Ask("")
		if answer == "" {
			//Exit if user does not input anything
			break
		}
		option, err := strconv.Atoi(answer)

		if option < 0 || option > 11 || err != nil {
			fmt.Println("Please input a number from 0-11")
			continue
		}
		if option == 0 {
			break
		}
		switch option {
		case 1, 2, 5, 8, 9, 10, 11:
			newValue := util.Ask("Enter value you would like to update to: (Enter nothing to remove)")
			updateStringValueInConfig(tracesConfig, option, newValue)
		case 3, 4:
			answer := util.Ask("Enter value you would like to update to: (Enter nothing to remove)")

			newValue, err := strconv.Atoi(answer)
			if err != nil {
				fmt.Println("Wrong Input! Input has go be a int.")
				continue
			} else {
				updateIntValueInConfig(tracesConfig, option, newValue)
			}

		case 6, 7:
			answer := util.Ask("Enter value you would like to update to: (Enter nothing to remove)")
			newValue, err := strconv.ParseBool(answer)
			if err != nil {
				fmt.Println("Wrong Input! Input has go be a bool")
			} else {
				updateBoolValueInConfig(tracesConfig, option, newValue)
			}
		}
	}
	return tracesConfig
}

// array of fields
func generateFieldOptions() []string {
	options := []string{
		"0: Keep this configuration and exit",
		"1: UDP BindAddress",
		"2: TCP BindAddress",
		"3: concurrency",
		"4: buffer_size_mb",
		"5: resource_arn",
		"6: local_mode",
		"7: insecure",
		"8: role_arn",
		"9: endpoint_override",
		"10: region_override",
		"11: proxy_override",
	}
	return options
}

func updateIntValueInConfig(tracesConfig *config.Traces, option int, value int) error {
	switch option {
	case 3:
		tracesConfig.Concurrency = value
	case 4:
		if value < 3 {
			fmt.Println("Input has to be bigger than 3! Value has been set to 3")
			tracesConfig.BufferSizeMB = 3
			return nil
		}
		tracesConfig.BufferSizeMB = value
	default:
		return errors.New("Unknown option")
	}
	return nil
}

func updateStringValueInConfig(tracesConfig *config.Traces, option int, value string) error {
	switch option {
	case 5:
		tracesConfig.ResourceArn = value
	case 9:
		tracesConfig.EndpointOverride = value
	case 10:
		tracesConfig.RegionOverride = value
	case 11:
		tracesConfig.ProxyOverride = value
	case 1:
		tracesConfig.TracesCollected.Xray.BindAddress = value
	case 2:
		tracesConfig.TracesCollected.Xray.TcpProxy.BindAddress = value
	case 8:
		if value == "" {
			tracesConfig.Credentials = nil
		} else if tracesConfig.Credentials == nil {
			tracesConfig.Credentials = &struct {
				RoleArn string `json:"role_arn,omitempty"`
			}{}
			tracesConfig.Credentials.RoleArn = value
		}
	default:
		return errors.New("Unknown Option")
	}
	return nil
}

func updateBoolValueInConfig(tracesConfig *config.Traces, option int, value bool) error {
	switch option {
	case 6:
		tracesConfig.LocalMode = value
	case 7:
		tracesConfig.Insecure = value

	default:
		return errors.New("Unknown Option")
	}
	return nil
}
func getCmdlines(processes []xraydaemonmigration.Process) []string {
	var cmdlines []string
	for i := 0; i < len(processes); i++ {
		curCmdline, err := processes[i].Cmdline()
		if err != nil {
			continue
		}
		cmdlines = append(cmdlines, curCmdline)

	}
	return cmdlines
}

func generateTracesConfiguration(ctx *runtime.Context) (*config.Traces, error) {
	var configFilePath string
	var tracesFile *config.Traces

	processes, err := xraydaemonmigration.FindAllDaemons()
	if err != nil {
		return nil, err
	}
	if len(processes) == 0 {
		yes := util.Yes(anyExistingDaemonConfiguration)
		if yes {
			return askUserInput(tracesFile, nil, yes)
		} else { //user can build config if they do not have a traces file
			return userBuildsTracesConfig(tracesFile), nil

		}
	}

	var chosenProcess xraydaemonmigration.Process
	if len(processes) > 1 {
		cmdlines := getCmdlines(processes)
		chosenCmdlineIndex := util.ChoiceIndex("Multiple active X-Ray Daemons detected.\nWhich of the configurations would you like to import?", 1, cmdlines)
		chosenProcess = processes[chosenCmdlineIndex]
	} else {
		fmt.Println("Detected X-Ray Daemon. The wizard will now attempt to import its configuration.")
		chosenProcess = processes[0]
	}

	configFilePath, err = xraydaemonmigration.FindConfigFile(chosenProcess)
	if err != nil {
		fmt.Println("Ran into error while trying to find Daemon Configurations. Using default traces configuration")
		err = json.Unmarshal(DefaultTracesConfigFile, &tracesFile)
		if err != nil {
			return nil, err
		}
		return tracesFile, nil
	} else if configFilePath == "" { //if user used command line to make configuration
		tracesFile, err = xraydaemonmigration.ConvertYamlToJson(nil, chosenProcess)
		if err != nil {
			fmt.Println("Failed to translate configuration to traces. Using default traces configuration")
			err = json.Unmarshal(DefaultTracesConfigFile, &tracesFile)
			if err != nil {
				return nil, err
			}
		}

		return tracesFile, nil
	}

	yamlFile, err := os.ReadFile(configFilePath)
	//incorrect configFilePath, user can decide to give path of default config will be used
	if err != nil {
		fmt.Println("Unable to import configuration from Detected Daemon")
		yes := util.Yes(anyExistingDaemonConfiguration)
		return askUserInput(tracesFile, chosenProcess, yes)
	}
	return xraydaemonmigration.ConvertYamlToJson(yamlFile, chosenProcess)
}

func askUserInput(tracesFile *config.Traces, chosenProcess xraydaemonmigration.Process, userHasImportConfig bool) (*config.Traces, error) {

	if userHasImportConfig {
		configFilePath := util.Ask(filePathXrayConfigQuestion)
		yamlFile, err := os.ReadFile(configFilePath)
		//error reading filepath given by user, using default config
		if err != nil {
			fmt.Println("There was an error reading X-Ray Daemon config file. Using default traces configurations")
			err := json.Unmarshal(DefaultTracesConfigFile, &tracesFile)
			return tracesFile, err
		}
		return xraydaemonmigration.ConvertYamlToJson(yamlFile, chosenProcess)
	} else { //user does not have exiting file, using default config
		fmt.Println("Using Default configuration file")
		err := json.Unmarshal(DefaultTracesConfigFile, &tracesFile)
		return tracesFile, err
	}
}
