// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentconfig

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/statsd"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

const (
	RUNASUSER_ROOT    = "root"
	RUNASUSER_CWAGENT = "cwagent"
	RUNASUSER_OTHERS  = "others"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
	whichRunAsUser(ctx, config)
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	return statsd.Processor
}

func whichRunAsUser(ctx *runtime.Context, config *data.Config) {
	if ctx.OsParameter == util.OsTypeWindows {
		return
	}

	answer := util.Choice("Which user are you planning to run the agent?",
		1,
		[]string{RUNASUSER_CWAGENT, RUNASUSER_ROOT, RUNASUSER_OTHERS})

	if answer == RUNASUSER_OTHERS {
		answer = util.Ask("Please specify your own user(remember the user must exist before the agent running):")
	}
	config.AgentConf().Runasuser = answer
}
