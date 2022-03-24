// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ssm

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"

	"github.com/aws/amazon-cloudwatch-agent/tool/util"

	"github.com/stretchr/testify/assert"
)

func TestProcessor_NextProcessor(t *testing.T) {
	nextProcessor := Processor.NextProcessor(nil, nil)
	assert.Equal(t, nil, nextProcessor)
}

func TestDetermineCreds(t *testing.T) {

}

func TestDetermineParameterStoreName(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := new(runtime.Context)

	ctx.OsParameter = util.OsTypeLinux
	testutil.Type(inputChan, "")
	parameterStoreName := determineParameterStoreName(ctx)
	assert.Equal(t, "AmazonCloudWatch-linux", parameterStoreName)

	ctx.OsParameter = util.OsTypeDarwin
	testutil.Type(inputChan, "")
	parameterStoreName = determineParameterStoreName(ctx)
	assert.Equal(t, "AmazonCloudWatch-darwin", parameterStoreName)

	ctx.OsParameter = util.OsTypeWindows
	testutil.Type(inputChan, "")
	parameterStoreName = determineParameterStoreName(ctx)
	assert.Equal(t, "AmazonCloudWatch-windows", parameterStoreName)

	testutil.Type(inputChan, "TestParameterStore")
	parameterStoreName = determineParameterStoreName(ctx)
	assert.Equal(t, "TestParameterStore", parameterStoreName)
}

func TestDetermineRegion(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := new(runtime.Context)

	// do not query ec2 metadata
	ctx.IsOnPrem = true

	testutil.Type(inputChan, "")
	region := determineRegion(ctx)
	assert.Equal(t, "us-east-1", region)

	testutil.Type(inputChan, "eu-west-1")
	region = determineRegion(ctx)
	assert.Equal(t, "eu-west-1", region)

}
