// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package serialization

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/ssm"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
	_, resultMap := config.ToMap(ctx)
	byteArray := util.SerializeResultMapToJsonByteArray(resultMap)
	var filepath string
	filepath = ctx.ConfigOutputPath
	if filepath == "" {
		filepath = util.ConfigFilePath()
	}
	util.SaveResultByteArrayToJsonFile(byteArray, filepath)
	fmt.Printf("Current config as follows:\n"+
		"%s\n"+
		"Please check the above content of the config.\n"+
		"The config file is also located at %s.\n"+
		"Edit it manually if needed.\n",
		string(byteArray),
		filepath)
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	return ssm.Processor
}
