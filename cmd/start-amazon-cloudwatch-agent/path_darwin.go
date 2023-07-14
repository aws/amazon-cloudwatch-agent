// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build darwin
// +build darwin

package main

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/config"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/context"
)

func setCTXOS(ctx *context.Context) {
	ctx.SetOs(config.OS_TYPE_DARWIN)
}
