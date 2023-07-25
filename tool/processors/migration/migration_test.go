// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/migration/linux"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/migration/windows"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

func TestProcessor_Process(t *testing.T) {
	ctx := new(runtime.Context)
	conf := new(data.Config)

	Processor.Process(ctx, conf)
	assert.Equal(t, new(runtime.Context), ctx)
	assert.Equal(t, new(data.Config), conf)
}

func TestProcessor_NextProcessor(t *testing.T) {
	ctx := new(runtime.Context)
	conf := new(data.Config)

	ctx.OsParameter = util.OsTypeLinux
	nextProcessor := Processor.NextProcessor(ctx, conf)
	assert.Equal(t, linux.Processor, nextProcessor)
	assert.Equal(t, new(data.Config), conf)

	ctx.OsParameter = util.OsTypeDarwin
	nextProcessor = Processor.NextProcessor(ctx, conf)
	assert.Equal(t, linux.Processor, nextProcessor)
	assert.Equal(t, new(data.Config), conf)

	ctx.OsParameter = util.OsTypeWindows
	nextProcessor = Processor.NextProcessor(ctx, conf)
	assert.Equal(t, windows.Processor, nextProcessor)
	assert.Equal(t, new(data.Config), conf)
}
