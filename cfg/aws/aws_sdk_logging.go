// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"github.com/aws/smithy-go/logging"
	"log"
	"strings"

	awsSDKV2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws"
)

// Hard coded strings that match actual variable names in the AWS SDK.
// I don't expect these names to change, or their meaning to change.
// Update this map if/when AWS SDK adds more levels (unlikely).
var stringToLevelMap map[string]aws.LogLevelType = map[string]aws.LogLevelType{
	LOG_DEBUG:                     aws.LogDebug,
	"LogDebugWithSigning":         aws.LogDebugWithSigning,
	"LogDebugWithHTTPBody":        aws.LogDebugWithHTTPBody,
	"LogDebugWithRequestRetries":  aws.LogDebugWithRequestRetries,
	"LogDebugWithRequestErrors":   aws.LogDebugWithRequestErrors,
	"LogDebugWithEventStreamBody": aws.LogDebugWithEventStreamBody,
}
var sdkv2StringToLevelMap map[string]awsSDKV2.ClientLogMode = map[string]awsSDKV2.ClientLogMode{
	"LogDebugWithSigning":         awsSDKV2.LogSigning,
	"LogDebugWithHTTPBody":        awsSDKV2.LogResponseWithBody,
	"LogDebugWithRequestRetries":  awsSDKV2.LogRetries,
	"LogDebugWithEventStreamBody": awsSDKV2.LogResponseEventMessage,
	"LogSigning":                  awsSDKV2.LogSigning,
	"LogRetries":                  awsSDKV2.LogRetries,
	"LogRequest":                  awsSDKV2.LogRequest,
	"LogRequestWithBody":          awsSDKV2.LogResponseWithBody,
	"LogResponse":                 awsSDKV2.LogResponse,
	"LogResponseWithBody":         awsSDKV2.LogRequestWithBody,
	"LogDeprecatedUsage":          awsSDKV2.LogDeprecatedUsage,
	"LogRequestEventMessage":      awsSDKV2.LogResponseEventMessage,
	"LogResponseEventMessage":     awsSDKV2.LogRequestEventMessage,
}
var (
	sdkLogLevel     aws.LogLevelType       = aws.LogOff
	sdkV2LogLevel   logging.Classification = logging.Warn
	sdkV2ClientMode awsSDKV2.ClientLogMode = 0
)

const (
	LOG_DEBUG = "LogDebug"
)

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

func SetSDKV2LogLevel(sdkLogLevelString string) {
	if strings.Contains(sdkLogLevelString, LOG_DEBUG) {
		sdkV2LogLevel = logging.Debug
	}

	var temp awsSDKV2.ClientLogMode = 0

	levels := strings.Split(sdkLogLevelString, "|")
	for _, v := range levels {
		trimmed := strings.TrimSpace(v)
		// If v not in map, then OR with 0 is harmless.
		temp |= sdkv2StringToLevelMap[trimmed]
	}

	sdkV2ClientMode = temp
}

// SDKLogLevel returns the single global value so it can be used in all
// AWS SDK calls scattered throughout the Agent.
func SDKLogLevel() *aws.LogLevelType {
	return &sdkLogLevel
}

func SDKV2ClientMode() awsSDKV2.ClientLogMode {
	return sdkV2ClientMode
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

// Logf logs the given classification and message to the underlying logger.
func (s SDKLogger) Logf(classification logging.Classification, format string, v ...interface{}) {
	if classification == logging.Debug && sdkV2LogLevel != logging.Debug {
		return
	}
	log.Printf("I! "+format, v)
}
