// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package retryer

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
)

type testLogger struct {
	debugs, infos, warns, errors []string
}

func (l *testLogger) Errorf(format string, args ...interface{}) {
	line := fmt.Sprintf(format, args...)
	l.errors = append(l.errors, line)
}

func (l *testLogger) Error(args ...interface{}) {
	line := fmt.Sprint(args...)
	l.errors = append(l.errors, line)
}

func (l *testLogger) Debugf(format string, args ...interface{}) {
	line := fmt.Sprintf(format, args...)
	l.debugs = append(l.debugs, line)
}

func (l *testLogger) Debug(args ...interface{}) {
	line := fmt.Sprint(args...)
	l.debugs = append(l.debugs, line)
}

func (l *testLogger) Warnf(format string, args ...interface{}) {
	line := fmt.Sprintf(format, args...)
	l.warns = append(l.warns, line)
}

func (l *testLogger) Warn(args ...interface{}) {
	line := fmt.Sprint(args...)
	l.warns = append(l.warns, line)
}

func (l *testLogger) Infof(format string, args ...interface{}) {
	line := fmt.Sprintf(format, args...)
	l.infos = append(l.infos, line)
}

func (l *testLogger) Info(args ...interface{}) {
	line := fmt.Sprint(args...)
	l.infos = append(l.infos, line)
}

func TestLogThrottleRetryerLogging(t *testing.T) {
	setup()
	defer tearDown()

	const throttleDebugLine = "AWS API call throttled: Operation: Test, Error: RequestLimitExceeded: Test AWS Error"
	const watchGoroutineExitLine = "LogThrottleRetryer watch throttle events goroutine exiting"
	const throttleSummaryLinePrefix = "AWS API call has been throttled"
	const throttleBatchSize = 100
	const totalThrottleCnt = throttleBatchSize * 2 // Test total 2 batches
	var throttleDetectedLine = fmt.Sprintf("AWS API call throttling detected, further throttling messages may be suppressed for up to %v depending on the log level, error message: Operation: Test, Error: RequestLimitExceeded: Test AWS Error", throttleReportTimeout)

	l := &testLogger{}
	r := NewLogThrottleRetryer(l)

	req := &request.Request{
		Error:     awserr.New("RequestLimitExceeded", "Test AWS Error", nil),
		Operation: &request.Operation{Name: "Test"},
	}

	// Generate 200 throttles with a time gap between
	for i := 0; i < throttleBatchSize; i++ {
		r.ShouldRetry(req)
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(1500 * time.Millisecond)

	for i := 0; i < throttleBatchSize; i++ {
		r.ShouldRetry(req)
		time.Sleep(10 * time.Millisecond)
	}

	r.Stop()
	time.Sleep(200 * time.Millisecond) // Wait a bit to collect all logs

	// Check the debug level log messages
	debugCnt := 0
	for _, d := range l.debugs {
		if d == throttleDebugLine {
			debugCnt++
		} else if d != watchGoroutineExitLine {
			t.Errorf("unexpected debug log found: %v", d)
		}
	}

	// Check the info level log messages
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
