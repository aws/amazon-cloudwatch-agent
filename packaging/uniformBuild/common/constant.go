// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type OS string

const TEST_REPO = "https://github.com/aws/amazon-cloudwatch-agent-test"
const MAIN_REPO = "https://github.com/aws/amazon-cloudwatch-agent.git"
const S3_INTEGRATION_BUCKET = "cloudwatch-agent-integration-bucket"
const BUILD_ARN = "arn:aws:iam::506463145083:instance-profile/Uniform-Build-Env-Instance-Profile"
const COMMAND_TRACKING_TIMEOUT = 20 * time.Minute
const COMMAND_TRACKING_INTERVAL = 1 * time.Second
const COMMAND_TRACKING_COUNT = int(COMMAND_TRACKING_TIMEOUT / COMMAND_TRACKING_INTERVAL)
const PLATFORM_KEY = "platform"
const GO_MOD_CACHE_DIR = "s3://cloudwatch-agent-integration-bucket/GoVendorCache/vendor.zip"
const (
	LINUX   OS = "linux"
	WINDOWS OS = "windows"
	MACOS   OS = "macos"
)

var OS_TO_INSTANCE_TYPES = map[OS]types.InstanceType{
	LINUX:   types.InstanceTypeT2Large,
	WINDOWS: types.InstanceTypeT2Large,
	MACOS:   types.InstanceTypeMac1Metal,
}
var SUPPORTED_OS = []OS{
	LINUX,
	WINDOWS,
	MACOS,
} //go doesn't let me create a slice from enum so this is the solution

var DEFAULT_INSTANCE_GUIDE = map[string]OS{
	"MainBuildEnv":      LINUX,
	"WindowsMSIPacker":  LINUX,
	"MacPkgMaker":       MACOS,
	"WindowsMSIBuilder": WINDOWS,
}
var LINUX_TEST_INSTANCE_GUIDE = map[string]OS{
	"MainBuildEnv": LINUX,
}
var MACOS_TEST_INSTANCE_GUIDE = map[string]OS{
	"MacPkgMaker": MACOS,
}
var WINDOWS_TEST_INSTANCE_GUIDE = map[string]OS{
	"WindowsMSIBuilder": WINDOWS,
}
