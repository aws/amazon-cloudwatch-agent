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

// countingEvent records Done() so undelivered events cannot be silently acknowledged.
type countingEvent struct {
	msg  string
	ts   time.Time
	done *atomic.Int32
}

func (e *countingEvent) Message() string { return e.msg }
func (e *countingEvent) Time() time.Time { return e.ts }
func (e *countingEvent) Done() {
	if e.done != nil {
		e.done.Add(1)
	}
}

// cwlServer serves the CWL JSON1.1 protocol. putErr selects the PutLogEvents outcome.
func cwlServer(putCalls *atomic.Int32, putErr string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := r.Header.Get("X-Amz-Target")
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		switch {
		case strings.Contains(target, "PutLogEvents"):
			putCalls.Add(1)
			if putErr != "" {
				w.Header().Set("X-Amzn-Errortype", putErr)
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"__type":"` + putErr + `","message":"injected"}`))
				return
			}
			_, _ = w.Write([]byte(`{}`))
		case strings.Contains(target, "DescribeLogGroups"):
			_, _ = w.Write([]byte(`{"logGroups":[]}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
}

func newPlugin(endpoint string, concurrency int) *CloudWatchLogs {
	return &CloudWatchLogs{
		Log:                testutil.Logger{Name: "test"},
		Region:             "us-east-1",
		AccessKey:          "access_key",
		SecretKey:          "secret_key",
		EndpointOverride:   endpoint,
		Concurrency:        concurrency,
		ForceFlushInterval: internal.Duration{Duration: 100 * time.Millisecond},
		cwDests:            sync.Map{},
	}
}

// R2: the collapse rebase unioned #2190's sharedRetryer/sharedClient with the poison-pill
// retryHeap/retryHeapProcessor in one struct. Both sets must be wired, and the shared pair
// must exist even when concurrency is disabled -- it is not gated on the retry heap.
func TestRebaseSeamStructUnion(t *testing.T) {
	var puts atomic.Int32
	srv := cwlServer(&puts, "")
	defer srv.Close()

	t.Run("ConcurrencyEnabled", func(t *testing.T) {
		c := newPlugin(srv.URL, 4)
		c.CreateDest("G", "S", -1, util.StandardLogGroupClass, nil)
		// #2190 half
		require.NotNil(t, c.sharedRetryer, "#2190 sharedRetryer lost in the rebase")
		require.NotNil(t, c.sharedClient, "#2190 sharedClient lost in the rebase")
		require.NotNil(t, c.targetManager)
		// poison-pill half
		require.NotNil(t, c.workerPool, "poison-pill workerPool lost in the rebase")
		require.NotNil(t, c.retryHeap, "poison-pill retryHeap lost in the rebase")
		require.NotNil(t, c.retryHeapProcessor, "poison-pill processor lost in the rebase")
		require.NotSame(t, c.sharedRetryer, c.retryHeapProcessor,
			"processor must not share the TargetManager's retryer")
		c.Close()
	})

	t.Run("ConcurrencyDisabled", func(t *testing.T) {
		c := newPlugin(srv.URL, 1)
		c.CreateDest("G", "S", -1, util.StandardLogGroupClass, nil)
		// #2190 must still apply with the retry heap switched off
		require.NotNil(t, c.sharedRetryer, "#2190 fix must not be gated on concurrency")
		require.NotNil(t, c.sharedClient)
		require.NotNil(t, c.targetManager)
		assert.Nil(t, c.workerPool)
		assert.Nil(t, c.retryHeap)
		assert.Nil(t, c.retryHeapProcessor)
		c.Close()
	})
}

// R3: shutdown must never acknowledge events that were never delivered -- finding #4's
// abandon()/drop() semantics at plugin scope. Every send fails, so Close() finalizes in-flight
// batches through those paths rather than done().
//
// SCOPE LIMIT: this does NOT verify the Close() ordering (processor -> pool -> heap). Established
// by mutation testing -- reverting to the pre-fix order still passes, because at Close() time the
// batch is mid-backoff, the final flush finds nothing ready, and no push-against-a-closed-heap
// occurs. The ordering only matters when a batch is ready for retry at the exact instant of
// shutdown, and that state cannot be forced from this package (logEventBatch is unexported in
// pusher). The ordering change is defense-in-depth with no automated coverage.
func TestRebaseSeamCloseDoesNotAckUndelivered(t *testing.T) {
	var puts atomic.Int32
	srv := cwlServer(&puts, "ServiceUnavailableException") // every send fails
	defer srv.Close()

	c := newPlugin(srv.URL, 4)
	dest := c.CreateDest("G", "S", -1, util.StandardLogGroupClass, nil)

	var doneCount atomic.Int32
	events := make([]logs.LogEvent, 0, 20)
	for i := 0; i < 20; i++ {
		events = append(events, &countingEvent{msg: "undelivered", ts: time.Now(), done: &doneCount})
	}
	require.NoError(t, dest.Publish(events))

	require.Eventually(t, func() bool { return puts.Load() > 0 }, 10*time.Second, 50*time.Millisecond,
		"test setup: expected at least one failed send attempt")

	closed := make(chan struct{})
	go func() { c.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(30 * time.Second):
		t.Fatal("Close() hung: shutdown ordering regression")
	}

	assert.Zero(t, doneCount.Load(),
		"Close() acknowledged %d events that never reached CloudWatch -- file offsets would "+
			"advance past them and they would be lost across restart", doneCount.Load())
}

// P8: concurrency <= 1 leaves retryHeap nil and the sender falls back to the synchronous
// path. main's default concurrency is 0, so this is the path existing installs run -- it must
// deliver and shut down cleanly with no nil dereference from the poison-pill wiring.
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
