// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go/logging"
	"github.com/stretchr/testify/assert"
)

func TestSetSDKLogLevel(t *testing.T) {
	testCases := []struct {
		input string
		want  aws.ClientLogMode
	}{
		// Invalid input
		{input: "FOO", want: aws.ClientLogMode(0)},
		// Wrong case.
		{input: "logrequest", want: aws.ClientLogMode(0)},
		// Extra char.
		{input: "LogRequest1", want: aws.ClientLogMode(0)},
		// Single match.
		{input: "LogRequest", want: aws.LogRequest},
		{input: "LogResponse", want: aws.LogResponse},
		{input: "LogSigning", want: aws.LogSigning},
		{input: "LogRequestWithBody", want: aws.LogRequestWithBody},
		{input: "LogResponseWithBody", want: aws.LogResponseWithBody},
		{input: "LogRetries", want: aws.LogRetries},
		{input: "LogRequestEventMessage", want: aws.LogRequestEventMessage},
		{input: "LogResponseEventMessage", want: aws.LogResponseEventMessage},
		{input: "LogDeprecatedUsage", want: aws.LogDeprecatedUsage},
		// v1 compatibility
		{input: "LogDebug", want: aws.LogRequest | aws.LogResponse},
		{input: "LogDebugWithSigning", want: aws.LogRequest | aws.LogResponse | aws.LogSigning},
		{input: "LogDebugWithHTTPBody", want: aws.LogRequestWithBody | aws.LogResponseWithBody},
		{input: "LogDebugWithRequestRetries", want: aws.LogRequest | aws.LogResponse | aws.LogRetries},
		{input: "LogDebugWithEventStreamBody", want: aws.LogRequestEventMessage | aws.LogResponseEventMessage},
		// Extra space around is allowed.
		{input: "   LogRequest  ", want: aws.LogRequest},
		// Multiple matches.
		{input: "LogRequest|LogResponse", want: aws.LogRequest | aws.LogResponse},
		{input: "  LogRequestWithBody  |  LogResponseWithBody  ", want: aws.LogRequestWithBody | aws.LogResponseWithBody},
		{input: "LogRetries|LogSigning", want: aws.LogRetries | aws.LogSigning},
		{input: "LogRequest|LogResponse|LogSigning", want: aws.LogRequest | aws.LogResponse | aws.LogSigning},
	}

	for _, testCase := range testCases {
		SetSDKLogLevel(testCase.input)
		assert.Equal(t, testCase.want, SDKLogLevel())
	}
}

func TestSDKLogger(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	logger := SDKLogger{}

	tests := []struct {
		classification logging.Classification
		expectedPrefix string
	}{
		{classification: logging.Debug, expectedPrefix: "D!"},
		{classification: logging.Warn, expectedPrefix: "W!"},
		{classification: logging.Classification("TEST"), expectedPrefix: "I!"},
	}

	for _, tt := range tests {
		t.Run(string(tt.classification), func(t *testing.T) {
			buf.Reset()
			logger.Logf(tt.classification, "test message: %s", "arg")

			output := buf.String()
			assert.Contains(t, output, tt.expectedPrefix)
			assert.Contains(t, output, "test message: arg")
		})
	}
}
