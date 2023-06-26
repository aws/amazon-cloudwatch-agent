// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/migration/linux"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/migration/windows"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	switch ctx.OsParameter {
	case util.OsTypeWindows:
		return windows.Processor
	default:
		return linux.Processor
	}
}
