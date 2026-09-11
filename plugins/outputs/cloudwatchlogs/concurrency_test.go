// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

// Both component sets must be wired: the shared retryer/client and the concurrent
// retry-heap components. The shared pair must exist even when concurrency is disabled --
// it is not gated on the retry heap.
func TestConcurrencyComponentsAllWired(t *testing.T) {
	var puts atomic.Int32
	srv := cwlServer(&puts, "")
	defer srv.Close()

	t.Run("ConcurrencyEnabled", func(t *testing.T) {
		c := newPlugin(srv.URL, 4)
		c.CreateDest("G", "S", -1, util.StandardLogGroupClass, nil)
		// shared retryer/client
		require.NotNil(t, c.sharedRetryer, "sharedRetryer must be wired")
		require.NotNil(t, c.sharedClient, "sharedClient must be wired")
		require.NotNil(t, c.targetManager)
		// concurrent retry-heap components
		require.NotNil(t, c.workerPool, "workerPool must be wired")
		require.NotNil(t, c.retryHeap, "retryHeap must be wired")
		require.NotNil(t, c.retryHeapProcessor, "processor must be wired")
		// Not asserted: the processor's retryer vs the TargetManager's sharedRetryer. The
		// processor does not expose its retryer, so there is nothing observable to compare.
		c.Close()
	})

	t.Run("ConcurrencyDisabled", func(t *testing.T) {
		c := newPlugin(srv.URL, 1)
		c.CreateDest("G", "S", -1, util.StandardLogGroupClass, nil)
		// shared retryer/client must exist even with the retry heap switched off
		require.NotNil(t, c.sharedRetryer, "sharedRetryer must not be gated on concurrency")
		require.NotNil(t, c.sharedClient)
		require.NotNil(t, c.targetManager)
		assert.Nil(t, c.workerPool)
		assert.Nil(t, c.retryHeap)
		assert.Nil(t, c.retryHeapProcessor)
		c.Close()
	})
}

// Concurrency <= 1 leaves retryHeap nil and the sender falls back to the synchronous path.
// main's default concurrency is 0, so this is the path existing installs run -- it must
// deliver and shut down cleanly with no nil dereference from the retry-heap wiring.
func TestSyncModeDefaultConcurrencyDelivers(t *testing.T) {
	for _, conc := range []int{0, 1} {
		t.Run("Concurrency"+strconv.Itoa(conc), func(t *testing.T) {
			var puts atomic.Int32
			srv := cwlServer(&puts, "")
			defer srv.Close()

			c := newPlugin(srv.URL, conc)
			dest := c.CreateDest("G", "S", -1, util.StandardLogGroupClass, nil)
			require.Nil(t, c.retryHeap, "retryHeap must be nil below the concurrency threshold")

			var doneCount atomic.Int32
			require.NoError(t, dest.Publish([]logs.LogEvent{
				&countingEvent{msg: "sync-mode", ts: time.Now(), done: &doneCount},
			}))

			require.Eventually(t, func() bool { return doneCount.Load() == 1 }, 15*time.Second,
				50*time.Millisecond, "synchronous path did not deliver (concurrency=%d)", conc)

			closed := make(chan struct{})
			go func() { c.Close(); close(closed) }()
			select {
			case <-closed:
			case <-time.After(20 * time.Second):
				t.Fatalf("Close() hung in synchronous mode (concurrency=%d)", conc)
			}
		})
	}
}

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
