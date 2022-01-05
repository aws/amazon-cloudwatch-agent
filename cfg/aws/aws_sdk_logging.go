// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
)

// Hard coded strings that match actual variable names in the AWS SDK.
// I don't expect these names to change, or their meaning to change.
// Update this map if/when AWS SDK adds more levels (unlikely).
var stringToLevelMap map[string]aws.LogLevelType = map[string]aws.LogLevelType{
	"LogDebug":                    aws.LogDebug,
	"LogDebugWithSigning":         aws.LogDebugWithSigning,
	"LogDebugWithHTTPBody":        aws.LogDebugWithHTTPBody,
	"LogDebugWithRequestRetries":  aws.LogDebugWithRequestRetries,
	"LogDebugWithRequestErrors":   aws.LogDebugWithRequestErrors,
	"LogDebugWithEventStreamBody": aws.LogDebugWithEventStreamBody,
}
var sdkLogLevel aws.LogLevelType = aws.LogOff

// SetSDKLogLevel sets the global log level which will be used in all AWS SDK calls.
// The levels are a bit field that is OR'd together.
// So the user can specify multiple levels and we OR them together.
// Example: "aws_sdk_log_level": "LogDebugWithSigning | LogDebugWithRequestErrors".
// JSON string value must contain the levels seperated by "|" and optionally whitespace.
func SetSDKLogLevel(sdkLogLevelString string) {
	var temp aws.LogLevelType = aws.LogOff

	levels := strings.Split(sdkLogLevelString, "|")
	for _, v := range levels {
		trimmed := strings.TrimSpace(v)
		// If v not in map, then OR with 0 is harmless.
		temp |= stringToLevelMap[trimmed]
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
	// Always use info logging level.
	tempSlice := append([]interface{}{"I!"}, args...)
	log.Println(tempSlice...)
}
