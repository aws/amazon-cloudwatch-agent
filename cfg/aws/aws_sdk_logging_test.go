// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/influxdata/wlog"
)

func TestSetSDKLogLevel(t *testing.T) {
	cases := []struct {
		wLogLevel         wlog.Level
		sdkLogLevelString string
		expectedVal       aws.LogLevelType
	}{
		// ENV VAR does not match
		{wlog.DEBUG, "FOO", aws.LogOff},
		{wlog.INFO, "FOO", aws.LogOff},
		{wlog.WARN, "FOO", aws.LogOff},
		{wlog.ERROR, "FOO", aws.LogOff},
		// ENV VAR matches, but wLogLevel is not DEBUG.
		{wlog.INFO, "DEBUG", aws.LogOff},
		{wlog.INFO, "DEBUG", aws.LogOff},
		{wlog.WARN, "DEBUG", aws.LogOff},
		{wlog.ERROR, "DEBUG", aws.LogOff},
		// ENV VAR matches, wLogLevel is DEBUG.
		{wlog.DEBUG, "LogDebug", aws.LogDebug},
		{wlog.DEBUG, "LogDebugWithEventStreamBody", aws.LogDebugWithEventStreamBody},
		{wlog.DEBUG, "LogDebugWithHTTPBody", aws.LogDebugWithHTTPBody},
		{wlog.DEBUG, "LogDebugWithSigning", aws.LogDebugWithSigning},
		// Multiple matches
		{wlog.DEBUG, "LogDebugWithEventStreamBody|LogDebugWithSigning",
			aws.LogDebugWithEventStreamBody | aws.LogDebugWithSigning},
		{wlog.DEBUG, "LogDebugWithEventStreamBody | LogDebugWithSigning | LogDebugWithHTTPBody",
			aws.LogDebugWithEventStreamBody | aws.LogDebugWithSigning | aws.LogDebugWithHTTPBody},
	}

	for _, tc := range cases {
		SetSDKLogLevel(tc.wLogLevel, tc.sdkLogLevelString)
		// check the internal var
		if *SDKLogLevel() != tc.expectedVal {
			t.Errorf("input: %v, actual: %v", tc, sdkLogLevel)
		}
	}
}
