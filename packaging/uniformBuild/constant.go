package main

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"time"
)

type OS string

const TEST_REPO = "https://github.com/aws/amazon-cloudwatch-agent-test"
const MAIN_REPO = "https://github.com/aws/amazon-cloudwatch-agent.git"
const S3_INTEGRATION_BUCKET = "uniform-build-env-test"
const BUILD_ARN = "arn:aws:iam::506463145083:instance-profile/Uniform-Build-Env-Instance-Profile"
const COMMAND_TRACKING_TIMEOUT = 20 * time.Minute
const COMMAND_TRACKING_INTERVAL = 1 * time.Second
const COMMAND_TRACKING_COUNT = int(COMMAND_TRACKING_TIMEOUT / COMMAND_TRACKING_INTERVAL)

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

const PLATFORM_KEY = "platform"
