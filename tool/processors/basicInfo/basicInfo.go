// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package basicInfo

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/agentconfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
	ensurePermission()
	welcome()
	whichOS(ctx)
	isEC2(ctx, config)
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	return agentconfig.Processor
}

func ensurePermission() {
	util.PermissionCheck()
}

func welcome() {
	fmt.Println("================================================================")
	fmt.Println("= Welcome to the Amazon CloudWatch Agent Configuration Manager =")
	fmt.Println("=                                                              =")
	fmt.Println("= CloudWatch Agent allows you to collect metrics and logs from =")
	fmt.Println("= your host and send them to CloudWatch. Additional CloudWatch =")
	fmt.Println("= charges may apply.                                           =")
	fmt.Println("================================================================")
}

func whichOS(ctx *runtime.Context) {
	defaultOption := 1
	opts := []string{util.OsTypeLinux, util.OsTypeWindows, util.OsTypeDarwin}
	for i, opt := range opts {
		if util.CurOS() == opt {
			defaultOption = i + 1
		}
	}
	answer := util.Choice("On which OS are you planning to use the agent?", defaultOption, opts)
	ctx.OsParameter = answer
}

func isEC2(ctx *runtime.Context, conf *data.Config) {
	defaultOption := 1
	defaultRegion := util.DefaultEC2Region()
	if defaultRegion == "" {
		defaultOption = 2
	}
	answer := util.Choice("Are you using EC2 or On-Premises hosts?",
		defaultOption,
		[]string{"EC2", "On-Premises"})
	ctx.IsOnPrem = answer == "On-Premises"
	if ctx.IsOnPrem {
		fmt.Println("Please make sure the credentials and region set correctly on your hosts.\n" +
			"Refer to http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html")
	}
}
