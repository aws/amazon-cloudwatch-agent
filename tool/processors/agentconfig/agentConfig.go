// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentconfig

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/statsd"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
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
		[]string{RUNASUSER_ROOT, RUNASUSER_CWAGENT, RUNASUSER_OTHERS})

	if answer == RUNASUSER_OTHERS {
		answer = util.Ask("Please specify your own user(remember the user must exist before the agent running):")
	}
	config.AgentConf().Runasuser = answer
}
