// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type retryHeapTestEvent struct {
	msg string
	ts  time.Time
}

func (e *retryHeapTestEvent) Message() string { return e.msg }
func (e *retryHeapTestEvent) Time() time.Time { return e.ts }
func (e *retryHeapTestEvent) Done()           {}

// TestRetryHeapProcessorHasTargetManager guards the initialization order in getDest:
// the RetryHeapProcessor captures targetManager by value, so targetManager must be built
// BEFORE the concurrency block. Otherwise the processor's sender holds a nil
// TargetManager and a retried batch hitting ResourceNotFoundException nil-panics in
// sender.Send (s.targetManager.InitTarget).
//
// PutLogEvents always returns ResourceNotFoundException: the first attempt fails (pusher's
// own sender) and the batch moves to the retry heap; the retry is handled by the
// PROCESSOR's sender. Each ResourceNotFound triggers InitTarget -> CreateLogStream, so
// >=2 CreateLogStream calls prove the retry path ran against a non-nil TargetManager.
// With the ordering bug this panics in a worker goroutine instead of failing gracefully.
func TestRetryHeapProcessorHasTargetManager(t *testing.T) {
	var createLogStreamCalls atomic.Int32
	var putLogEventsCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := r.Header.Get("X-Amz-Target")
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		switch {
		case strings.Contains(target, "PutLogEvents"):
			putLogEventsCalls.Add(1)
			w.Header().Set("X-Amzn-Errortype", "ResourceNotFoundException")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"__type":"ResourceNotFoundException","message":"log stream does not exist"}`))
		case strings.Contains(target, "CreateLogStream"):
			createLogStreamCalls.Add(1)
			_, _ = w.Write([]byte(`{}`))
		case strings.Contains(target, "DescribeLogGroups"):
			_, _ = w.Write([]byte(`{"logGroups":[]}`))
		default: // CreateLogGroup, PutRetentionPolicy, ...
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	c := &CloudWatchLogs{
		Log:                testutil.Logger{Name: "test"},
		Region:             "us-east-1",
		AccessKey:          "access_key",
		SecretKey:          "secret_key",
		EndpointOverride:   srv.URL,
		Concurrency:        2, // > 1 enables the worker pool + retry heap
		ForceFlushInterval: internal.Duration{Duration: 100 * time.Millisecond},
		cwDests:            sync.Map{},
	}

	dest := c.CreateDest("G", "S", -1, util.StandardLogGroupClass, nil)
	require.NotNil(t, c.retryHeapProcessor, "retry heap processor should exist with concurrency > 1")
	require.NotNil(t, c.targetManager, "targetManager must be initialized in getDest")

	require.NoError(t, dest.Publish([]logs.LogEvent{
		&retryHeapTestEvent{msg: "msg", ts: time.Now()},
	}))

	// Wait for several retry cycles. Each retry re-enters sender.Send, which calls
	// targetManager.InitTarget on ResourceNotFoundException. With a nil TargetManager the
	// FIRST retry panics in a worker goroutine (killing the test process), so sustained
	// progress past the initial attempt is the signal that the ordering is correct.
	// InitTarget caches per target for cacheTTL (5s), so CreateLogStream is NOT re-called
	// on every retry -- do not assert on its count.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && putLogEventsCalls.Load() < 4 {
		time.Sleep(50 * time.Millisecond)
	}

	t.Logf("PutLogEvents=%d CreateLogStream=%d", putLogEventsCalls.Load(), createLogStreamCalls.Load())
	require.GreaterOrEqual(t, createLogStreamCalls.Load(), int32(1),
		"InitTarget should have created the stream at least once")
	require.GreaterOrEqual(t, putLogEventsCalls.Load(), int32(4),
		"retries must keep progressing through the retry heap; a nil TargetManager in the "+
			"processor's sender panics on the first retry (getDest built the retry heap "+
			"before targetManager)")

	c.Close()
}
