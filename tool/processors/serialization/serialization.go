// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package serialization

import (
	"fmt"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/ssm"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
	_, resultMap := config.ToMap(ctx)
	byteArray := util.SerializeResultMapToJsonByteArray(resultMap)
	util.SaveResultByteArrayToJsonFile(byteArray)
	fmt.Printf("Current config as follows:\n"+
		"%s\n"+
		"Please check the above content of the config.\n"+
		"The config file is also located at %s.\n"+
		"Edit it manually if needed.\n",
		string(byteArray),
		util.ConfigFilePath())
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	return ssm.Processor
}
