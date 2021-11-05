// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/influxdata/wlog"
)

var sdkLogLevel aws.LogLevelType = aws.LogOff

// SetSDKLogLevel sets the global log level which will be used in all AWS SDK calls.
// Example usage: set `debug = true` and `aws_sdk_log_level = LogDebug` in config.json.
func SetSDKLogLevel(wLogLevel wlog.Level, sdkLogLevelString string) {
	if wLogLevel != wlog.DEBUG {
		sdkLogLevel = aws.LogOff
		return
	}
	var temp aws.LogLevelType = aws.LogOff
	// Hard coded strings that match actual variable names in the AWS SDK.
	// I don't expect these names to change, or their meaning to change.
	// This code may need updates if the SDK adds more levels (unlikely).
	// Please note that the levels are a bit field that is OR'd together.
	if strings.Contains(sdkLogLevelString, "LogDebug") {
		temp |= aws.LogDebug
	}
	if strings.Contains(sdkLogLevelString, "LogDebugWithSigning") {
		temp |= aws.LogDebugWithSigning
	}
	if strings.Contains(sdkLogLevelString, "LogDebugWithHTTPBody") {
		temp |= aws.LogDebugWithHTTPBody
	}
	if strings.Contains(sdkLogLevelString, "LogDebugWithRequestErrors") {
		temp |= aws.LogDebugWithRequestErrors
	}
	if strings.Contains(sdkLogLevelString, "LogDebugWithEventStreamBody") {
		temp |= aws.LogDebugWithEventStreamBody
	}

	sdkLogLevel = temp
}

// SDKLogLevel returns the single global value so it can be used in all
// AWS SDK calls scattered throughout the Agent.
func SDKLogLevel() *aws.LogLevelType {
	return &sdkLogLevel
}

// SDKLogger implements the aws.Logger interface.
type SDKLogger struct {
}

// Log is the only method in the aws.Logger interface.
func (SDKLogger) Log(args ...interface{}) {
	// Always use debug logging level.
	log.Println("D! ", args)
}
