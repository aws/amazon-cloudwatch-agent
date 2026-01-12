// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go/logging"
)

// Hard coded strings that match actual variable names in the AWS SDK.
// I don't expect these names to change, or their meaning to change.
// Update this map if/when AWS SDK adds more levels (unlikely).
var stringToLevelMap = map[string]aws.ClientLogMode{
	// AWS SDK v2 Levels
	"LogRequest":              aws.LogRequest,
	"LogResponse":             aws.LogResponse,
	"LogSigning":              aws.LogSigning,
	"LogRequestWithBody":      aws.LogRequestWithBody,
	"LogResponseWithBody":     aws.LogResponseWithBody,
	"LogRetries":              aws.LogRetries,
	"LogRequestEventMessage":  aws.LogRequestEventMessage,
	"LogResponseEventMessage": aws.LogResponseEventMessage,
	"LogDeprecatedUsage":      aws.LogDeprecatedUsage,
	// AWS SDK v1 Levels
	"LogDebug":                    aws.LogRequest | aws.LogResponse,
	"LogDebugWithSigning":         aws.LogRequest | aws.LogResponse | aws.LogSigning,
	"LogDebugWithHTTPBody":        aws.LogRequestWithBody | aws.LogResponseWithBody,
	"LogDebugWithRequestRetries":  aws.LogRequest | aws.LogResponse | aws.LogRetries,
	"LogDebugWithRequestErrors":   aws.LogRequest | aws.LogResponse, // no equivalent in AWS SDK v2
	"LogDebugWithEventStreamBody": aws.LogRequestEventMessage | aws.LogResponseEventMessage,
}
var sdkLogLevel aws.ClientLogMode

// SetSDKLogLevel sets the global log level which will be used in all AWS SDK calls.
// The levels are a bit field that is OR'd together.
// So the user can specify multiple levels and we OR them together.
// Example: "aws_sdk_log_level": "LogDebugWithSigning | LogDebugWithRequestErrors".
// JSON string value must contain the levels separated by "|" and optionally whitespace.
func SetSDKLogLevel(sdkLogLevelString string) {
	var temp aws.ClientLogMode

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
func SDKLogLevel() aws.ClientLogMode {
	return sdkLogLevel
}

// SDKLogger implements the aws.Logger interface.
type SDKLogger struct {
}

var _ logging.Logger = (*SDKLogger)(nil)

func (SDKLogger) Logf(classification logging.Classification, format string, args ...interface{}) {
	logLevelPrefix := "I!"
	switch classification {
	case logging.Debug:
		logLevelPrefix = "D!"
	case logging.Warn:
		logLevelPrefix = "W!"
	}
	log.Printf(fmt.Sprintf("%s %s", logLevelPrefix, format), args...)
}
