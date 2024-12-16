// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package wizard

import (
	"bufio"
	"fmt"
	"os"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/basicInfo"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/migration/linux"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/serialization"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/tracesconfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/stdin"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
	wizardflags "github.com/aws/amazon-cloudwatch-agent/tool/wizard/flags"
)

type IMainProcessor interface {
	VerifyProcessor(processor interface{})
}

type MainProcessorStruct struct{}

var MainProcessorGlobal IMainProcessor = &MainProcessorStruct{}

type Params struct {
	IsNonInteractiveWindowsMigration bool
	IsNonInteractiveLinuxMigration   bool
	TracesOnly                       bool
	UseParameterStore                bool
	IsNonInteractiveXrayMigration    bool
	ConfigFilePath                   string
	ConfigOutputPath                 string
	ParameterStoreName               string
	ParameterStoreRegion             string
}

func init() {
	stdin.Scanln = func(a ...interface{}) (n int, err error) {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		if len(a) > 0 {
			*a[0].(*string) = scanner.Text()
			n = len(*a[0].(*string))
		}
		err = scanner.Err()
		return
	}
	processors.StartProcessor = basicInfo.Processor
}

func RunWizard(params Params) error {
	if params.IsNonInteractiveWindowsMigration {
		addWindowsMigrationInputs(
			params.ConfigFilePath,
			params.ParameterStoreName,
			params.ParameterStoreRegion,
			params.UseParameterStore,
		)
	} else if params.IsNonInteractiveLinuxMigration {
		ctx := new(runtime.Context)
		config := new(data.Config)
		ctx.HasExistingLinuxConfig = true
		ctx.ConfigFilePath = params.ConfigFilePath
		if ctx.ConfigFilePath == "" {
			ctx.ConfigFilePath = wizardflags.DefaultFilePathLinuxConfiguration
		}
		process(ctx, config, linux.Processor, serialization.Processor)
		return nil
	} else if params.TracesOnly {
		ctx := new(runtime.Context)
		config := new(data.Config)
		ctx.TracesOnly = true
		ctx.ConfigOutputPath = params.ConfigOutputPath
		ctx.NonInteractiveXrayMigration = params.IsNonInteractiveXrayMigration
		process(ctx, config, tracesconfig.Processor, serialization.Processor)
		return nil
	}

	startProcessing(params.ConfigOutputPath, params.IsNonInteractiveWindowsMigration, params.IsNonInteractiveXrayMigration)
	return nil
}

func RunWizardFromFlags(flags map[string]*string) error {
	params := Params{
		IsNonInteractiveWindowsMigration: *flags["is-non-interactive-windows-migration"] == "true",
		IsNonInteractiveLinuxMigration:   *flags["is-non-interactive-linux-migration"] == "true",
		TracesOnly:                       *flags["traces-only"] == "true",
		UseParameterStore:                *flags["use-parameter-store"] == "true",
		IsNonInteractiveXrayMigration:    *flags["non-interactive-xray-migration"] == "true",
		ConfigFilePath:                   *flags["config-file-path"],
		ConfigOutputPath:                 *flags["config-output-path"],
		ParameterStoreName:               *flags["parameter-store-name"],
		ParameterStoreRegion:             *flags["parameter-store-region"],
	}
	return RunWizard(params)
}

func addWindowsMigrationInputs(configFilePath string, parameterStoreName string, parameterStoreRegion string, useParameterStore bool) {
	inputChan := testutil.SetUpTestInputStream()
	if useParameterStore {
		testutil.Type(inputChan, "2", "1", "2", "1", configFilePath, "1", parameterStoreName, parameterStoreRegion, "1")
	} else {
		testutil.Type(inputChan, "2", "1", "2", "1", configFilePath, "2")
	}
}

func process(ctx *runtime.Context, config *data.Config, processors ...processors.Processor) {
	for _, processor := range processors {
		processor.Process(ctx, config)
	}
}

func startProcessing(configOutputPath string, isNonInteractiveWindowsMigration, isNonInteractiveXrayMigration bool) {
	ctx := &runtime.Context{
		ConfigOutputPath:               configOutputPath,
		WindowsNonInteractiveMigration: isNonInteractiveWindowsMigration,
		NonInteractiveXrayMigration:    isNonInteractiveXrayMigration,
	}
	config := &data.Config{}
	var processor interface{}
	processor = processors.StartProcessor
	for {
		if processor == nil {
			if util.CurOS() == util.OsTypeWindows && !isNonInteractiveWindowsMigration {
				util.EnterToExit()
			}
			fmt.Println("Program exits now.")
			break
		}
		MainProcessorGlobal.VerifyProcessor(processor) // For testing purposes
		processor.(processors.Processor).Process(ctx, config)
		processor = processor.(processors.Processor).NextProcessor(ctx, config)
	}
}

func (p *MainProcessorStruct) VerifyProcessor(processor interface{}) {
}
