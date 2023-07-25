// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package basicInfo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/agentconfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

func TestProcessor_Process(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := new(runtime.Context)
	conf := new(data.Config)

	testutil.Type(inputChan, "", "1")
	Processor.Process(ctx, conf)
	assert.Equal(t, util.CurOS(), ctx.OsParameter)
	assert.Equal(t, false, ctx.IsOnPrem)

	testutil.Type(inputChan, "1", "1")
	Processor.Process(ctx, conf)
	assert.Equal(t, util.OsTypeLinux, ctx.OsParameter)
	assert.Equal(t, false, ctx.IsOnPrem)

	testutil.Type(inputChan, "2", "2", "", "AK", "SK")
	Processor.Process(ctx, conf)
	assert.Equal(t, util.OsTypeWindows, ctx.OsParameter)

	assert.Equal(t, true, ctx.IsOnPrem)
}

func TestProcessor_NextProcessor(t *testing.T) {
	assert.Equal(t, agentconfig.Processor, Processor.NextProcessor(nil, nil))
}

func TestWhichOS(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := new(runtime.Context)

	testutil.Type(inputChan, "1")
	whichOS(ctx)
	assert.Equal(t, util.OsTypeLinux, ctx.OsParameter)

	testutil.Type(inputChan, "2")
	whichOS(ctx)
	assert.Equal(t, util.OsTypeWindows, ctx.OsParameter)

	testutil.Type(inputChan, "3")
	whichOS(ctx)
	assert.Equal(t, util.OsTypeDarwin, ctx.OsParameter)
}

func TestIsEC2(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := new(runtime.Context)
	conf := new(data.Config)

	//ec2
	testutil.Type(inputChan, "1")
	isEC2(ctx, conf)
	assert.Equal(t, false, ctx.IsOnPrem)

	//on-prem
	testutil.Type(inputChan, "2")
	isEC2(ctx, conf)
	assert.Equal(t, true, ctx.IsOnPrem)

}
