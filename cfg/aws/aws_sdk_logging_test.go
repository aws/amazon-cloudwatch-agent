// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
)

func TestSetSDKLogLevel(t *testing.T) {
	cases := []struct {
		sdkLogLevelString string
		expectedVal       aws.LogLevelType
	}{
		// sdkLogLevelString does not match
		{"FOO", aws.LogOff},
		// Wrong case.
		{"logDEBUG", aws.LogOff},
		// Extra char.
		{"LogDebug1", aws.LogOff},
		// Single match.
		{"LogDebug", aws.LogDebug},
		{"LogDebugWithEventStreamBody", aws.LogDebugWithEventStreamBody},
		{"LogDebugWithHTTPBody", aws.LogDebugWithHTTPBody},
		{"LogDebugWithRequestRetries", aws.LogDebugWithRequestRetries},
		{"LogDebugWithRequestErrors", aws.LogDebugWithRequestErrors},
		{"LogDebugWithEventStreamBody", aws.LogDebugWithEventStreamBody},
		// Extra space around is allowed.
		{"   LogDebug  ", aws.LogDebug},
		// Multiple matches.
		{"LogDebugWithEventStreamBody|LogDebugWithHTTPBody",
			aws.LogDebugWithEventStreamBody | aws.LogDebugWithHTTPBody},
		{"  LogDebugWithHTTPBody  |  LogDebugWithEventStreamBody  ",
			aws.LogDebugWithEventStreamBody | aws.LogDebugWithHTTPBody},
		{"LogDebugWithRequestRetries|LogDebugWithEventStreamBody",
			aws.LogDebugWithEventStreamBody | aws.LogDebugWithRequestRetries},
		{"LogDebugWithRequestRetries|LogDebugWithRequestErrors",
			aws.LogDebugWithRequestRetries | aws.LogDebugWithRequestErrors},
		{"LogDebugWithRequestRetries|LogDebugWithRequestErrors|LogDebugWithEventStreamBody",
			aws.LogDebugWithRequestRetries | aws.LogDebugWithRequestErrors | aws.LogDebugWithEventStreamBody},
	}

	for _, tc := range cases {
		SetSDKLogLevel(tc.sdkLogLevelString)
		// check the internal var
		if *SDKLogLevel() != tc.expectedVal {
			t.Errorf("input: %v, actual: %v", tc, sdkLogLevel)
		}
	}
}
