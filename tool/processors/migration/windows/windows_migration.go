// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/defaultConfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/ssm"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

var Processor processors.Processor = &processor{}

type processor struct{}

const (
	anyExistingLinuxConfigQuestion      = "Do you have any existing CloudWatch Log Agent configuration file to import for migration?"
	filePathWindowsConfigQuestion       = "What is the file path for the existing Windows CloudWatch log agent configuration file?"
	DefaultFilePathWindowsConfiguration = "C:\\Program Files\\Amazon\\SSM\\Plugins\\awsCloudWatch\\AWS.EC2.Windows.CloudWatch.json"
)

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	if util.No(anyExistingLinuxConfigQuestion) {
		migrateOldAgentConfig()
		return ssm.Processor
	}
	return defaultConfig.Processor
}

func migrateOldAgentConfig() {
	// 1 - parse the old config
	var oldConfig OldSsmCwConfig
	absPath := util.AskWithDefault(filePathWindowsConfigQuestion, DefaultFilePathWindowsConfiguration)
	if file, err := os.ReadFile(absPath); err == nil {
		if err := json.Unmarshal(file, &oldConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse the provided configuration file. Error details: %v", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Failed to read the provided configuration file. Error details: %v", err)
		os.Exit(1)
	}

	// 2 - map to the new config
	newConfig := MapOldWindowsConfigToNewConfig(oldConfig)

	// 3 - marshall the new config object to string
	if newConfigJson, err := json.Marshal(newConfig); err == nil {
		util.SaveResultByteArrayToJsonFile(newConfigJson, util.ConfigFilePath())
	} else {
		fmt.Fprintf(os.Stderr, "Failed to produce the new configuration file. Error details: %v", err)
		os.Exit(1)
	}
}
