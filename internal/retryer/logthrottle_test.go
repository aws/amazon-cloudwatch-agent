// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package retryer

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/smithy-go"
)

type testLogger struct {
	debugs, infos, warns, errors []string
}

func (l *testLogger) Errorf(format string, args ...interface{}) {
	l.errors = append(l.errors, fmt.Sprintf(format, args...))
}

func (l *testLogger) Error(args ...interface{}) {
	l.errors = append(l.errors, fmt.Sprint(args...))
}

func (l *testLogger) Debugf(format string, args ...interface{}) {
	l.debugs = append(l.debugs, fmt.Sprintf(format, args...))
}

func (l *testLogger) Debug(args ...interface{}) {
	l.debugs = append(l.debugs, fmt.Sprint(args...))
}

func (l *testLogger) Warnf(format string, args ...interface{}) {
	l.warns = append(l.warns, fmt.Sprintf(format, args...))
}

func (l *testLogger) Warn(args ...interface{}) {
	l.warns = append(l.warns, fmt.Sprint(args...))
}

func (l *testLogger) Infof(format string, args ...interface{}) {
	l.infos = append(l.infos, fmt.Sprintf(format, args...))
}

func (l *testLogger) Info(args ...interface{}) {
	l.infos = append(l.infos, fmt.Sprint(args...))
}

func TestLogThrottleRetryerLogging(t *testing.T) {
	setup()
	defer tearDown()

	const throttleDebugLine = "AWS API call throttled: Operation: PutMetricData, Error: operation error CloudWatch: PutMetricData, LimitExceededException: Request limit exceeded"
	const watchGoroutineExitLine = "LogThrottleRetryer watch throttle events goroutine exiting"
	const throttleSummaryLinePrefix = "AWS API call has been throttled"
	const throttleBatchSize = 100
	const totalThrottleCnt = throttleBatchSize * 2

	err := &smithy.OperationError{
		ServiceID:     "CloudWatch",
		OperationName: "PutMetricData",
		Err: &types.LimitExceededException{
			Message: aws.String("Request limit exceeded"),
		},
	}

	l := &testLogger{}
	r := NewLogThrottleRetryer(l)

	throttleDetectedLine := fmt.Sprintf("AWS API call throttling detected, further throttling messages may be suppressed for up to %v depending on the log level, error message: Operation: PutMetricData, Error: %v", throttleReportTimeout, err)

	// Generate 200 throttles with a time gap between
	for i := 0; i < throttleBatchSize; i++ {
		r.IsErrorRetryable(err)
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(1500 * time.Millisecond)

	for i := 0; i < throttleBatchSize; i++ {
		r.IsErrorRetryable(err)
		time.Sleep(10 * time.Millisecond)
	}

	r.Stop()
	time.Sleep(200 * time.Millisecond)

	// Check debug level log messages
	debugCnt := 0
	for _, d := range l.debugs {
		if d == throttleDebugLine {
			debugCnt++
		} else if d != watchGoroutineExitLine {
			t.Errorf("unexpected debug log found: %v", d)
		}
	}

	// Check info level log messages
	detectCnt := 0
	throttleCnt := 0
	for _, info := range l.infos {
		if info == throttleDetectedLine {
			detectCnt++
		} else if strings.HasPrefix(info, throttleSummaryLinePrefix) {
			n := 0
			fmt.Sscanf(info, throttleSummaryLinePrefix+" %d", &n)
			throttleCnt += n
		}
	}

	if detectCnt+debugCnt != totalThrottleCnt {
		t.Errorf("wrong number of throttle detected log found, expecting %v, got %v", totalThrottleCnt, detectCnt+debugCnt)
	}
	if throttleCnt != totalThrottleCnt {
		t.Errorf("wrong number of throttle count sum reported from info logs, expecting %v, got %v", totalThrottleCnt, throttleCnt)
	}
}

func setup() {
	throttleReportTimeout = 400 * time.Millisecond
	throttleReportCheckPeriod = 50 * time.Millisecond
}

func tearDown() {
	throttleReportTimeout = 1 * time.Minute
	throttleReportCheckPeriod = 5 * time.Second
}
