// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsservice

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/xray"
)

var (
	ctx        = context.Background()
	awsCfg, _  = config.LoadDefaultConfig(ctx)
	XrayClient = xray.NewFromConfig(awsCfg)
)
