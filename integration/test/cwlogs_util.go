// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package test

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

var (
	cwClient *cloudwatchlogs.Client
	ctx context.Context
	LogGroupName  = "cloudwatch-agent-integ-test"
	LogStreamName = "test-logs"
)

func GetClient() (*cloudwatchlogs.Client, context.Context, error) {
	if cwClient == nil {
		ctx = context.Background()
		c, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, nil, err
		}

		cwClient = cloudwatchlogs.NewFromConfig(c)
	}
	return cwClient, ctx, nil
}
