// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tracesconfig

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data/config"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/serialization"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/xraydaemonmigration"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, cfg *data.Config) {
	//skip if linux or windows noninteractive migration
	if ctx.WindowsNonInteractiveMigration {
		return
	}

	if !ctx.TracesOnly {
		yes := util.Yes("Do you want to add traces to your configuration?")
		if !yes {
			return
		}
	}
	newTraces, err := generateTracesConfiguration(ctx)
	if err != nil || newTraces == nil {
		return
	}
	cfg.TracesConfig = newTraces
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	return serialization.Processor
}

//go:embed configtraces.json
var DefaultTracesConfigFile []byte

const (
	anyExistingDaemonConfiguration = "Do you have any existing X-Ray Daemon configuration file to import for migration?"
	filePathXrayConfigQuestion     = "What is the file path for the existing X-Ray Daemon configuration file?"
)

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
		return askUserInput(tracesFile, nil)
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
		err = json.Unmarshal(DefaultTracesConfigFile, &tracesFile)
		if err != nil {
			return nil, err
		}
		return tracesFile, nil
	} else if configFilePath == "" { //if user used command line to make configuration
		tracesFile, err = xraydaemonmigration.ConvertYamlToJson(nil, chosenProcess)
		if err != nil {
			return nil, err
		}
		return tracesFile, nil
	}

	yamlFile, err := os.ReadFile(configFilePath)
	//incorrect configFilePath, user can decide to give path of default config will be used
	if err != nil {
		fmt.Println("Unable to import configuration from Detected Daemon")
		return askUserInput(tracesFile, chosenProcess)
	}
	return xraydaemonmigration.ConvertYamlToJson(yamlFile, chosenProcess)
}

func askUserInput(tracesFile *config.Traces, chosenProcess xraydaemonmigration.Process) (*config.Traces, error) {
	yes := util.Yes(anyExistingDaemonConfiguration)
	if yes {
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
