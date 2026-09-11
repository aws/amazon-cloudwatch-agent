// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"io"
	"net/http"
	"net/http/httptest"
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

// Shutdown must never acknowledge events that were never delivered: abandon()/drop()
// semantics at plugin scope. Every send fails, so Close() finalizes in-flight batches
// through those paths rather than done().
//
// SCOPE LIMIT: this does NOT verify the Close() ordering (processor -> pool -> heap). Established
// by mutation testing -- reverting to the pre-fix order still passes, because at Close() time the
// batch is mid-backoff, the final flush finds nothing ready, and no push-against-a-closed-heap
// occurs. The ordering only matters when a batch is ready for retry at the exact instant of
// shutdown, and that state cannot be forced from this package (logEventBatch is unexported in
// pusher). The ordering change is defense-in-depth with no automated coverage.
func TestCloseDoesNotAckUndeliveredEvents(t *testing.T) {
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

// A persistently failing target latches the per-target circuit
// breaker, which parks the queue's send loop in waitIfHalted. That loop drains eventsCh,
// so eventsCh (cap 100) fills, AddEvent's blocking send blocks, and Publish blocks while
// holding cd.Lock(). Close() -> cwDest.Stop() then needs the same cd.Lock(), so it can
// never reach queue.Stop() to close stopCh -- the only thing that releases waitIfHalted.
//
// Chain: queue.go:284 halt -> queue.go:182 waitIfHalted -> queue.go:169 mergeChan send
// -> queue.go:90 eventsCh send -> cloudwatchlogs.go:295 cd.Lock held by Publish
// -> cloudwatchlogs.go:104 d.Stop() wants cd.Lock -> queue.Stop never runs.
func TestCloseDoesNotWedgeOnPersistentlyHaltedTarget(t *testing.T) {
	var puts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		if !strings.Contains(r.Header.Get("X-Amz-Target"), "PutLogEvents") {
			io.WriteString(w, `{}`)
			return
		}
		puts.Add(1)
		// Retryable-by-sender, not retried by the SDK: every batch fails forever, so the
		// breaker latches and never clears.
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"__type":"OperationAbortedException","message":"forced"}`)
	}))
	defer srv.Close()

	c := &CloudWatchLogs{
		Log:                testutil.Logger{Name: "test"},
		Region:             "us-east-1",
		AccessKey:          "access_key",
		SecretKey:          "secret_key",
		EndpointOverride:   srv.URL,
		Concurrency:        2,
		ForceFlushInterval: internal.Duration{Duration: 50 * time.Millisecond},
		cwDests:            sync.Map{},
	}

	dest := c.CreateDest("G", "S", -1, util.StandardLogGroupClass, nil).(*cwDest)
	require.NotNil(t, c.retryHeap, "retry heap must be active at concurrency > 1")

	// Keep publishing so eventsCh (cap 100) fills once the send loop is parked.
	pubStop := make(chan struct{})
	var pubDone sync.WaitGroup
	pubDone.Add(1)
	go func() {
		defer pubDone.Done()
		for i := 0; ; i++ {
			select {
			case <-pubStop:
				return
			default:
			}
			// Blocks (holding cd.Lock) once eventsCh is full -- that is the wedge.
			_ = dest.Publish([]logs.LogEvent{
				&retryHeapTestEvent{msg: "payload", ts: time.Now()},
			})
		}
	}()

	// Wait until the target has actually failed at least once (breaker latched).
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && puts.Load() == 0 {
		time.Sleep(25 * time.Millisecond)
	}
	require.Positive(t, puts.Load(), "expected at least one failed PutLogEvents to latch the breaker")
	time.Sleep(2 * time.Second) // let eventsCh fill behind the parked send loop

	closed := make(chan struct{})
	go func() {
		c.Close()
		close(closed)
	}()

	select {
	case <-closed:
		// Close completed: shutdown is not wedged by the halted target.
	case <-time.After(45 * time.Second):
		close(pubStop)
		t.Fatal("Close() did not return within 45s: a persistently halted target wedged " +
			"shutdown (Publish holds cd.Lock while blocked in AddEvent, so cwDest.Stop " +
			"cannot acquire it and queue.stopCh is never closed)")
	}

	close(pubStop)
	// Publish returns ErrOutputStopped once the dest is stopped, so the producer unwinds.
	pubDone.Wait()
}
