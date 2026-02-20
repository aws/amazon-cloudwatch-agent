// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"sync"
	"testing"
	"time"

	"github.com/influxdata/telegraf/testutil"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/cloudwatchlogs/internal/pusher"
)

type blockingQueue struct {
	eventsCh chan logs.LogEvent
	stopped  bool
}

func (q *blockingQueue) AddEvent(e logs.LogEvent) {
	q.eventsCh <- e // blocks when channel is full
}

func (q *blockingQueue) AddEventNonBlocking(e logs.LogEvent) {}

func (q *blockingQueue) Stop() {
	if !q.stopped {
		q.stopped = true
	}
}

// TestPublishDoesNotBlockGetDest verifies that when Publish blocks on a
// full queue while holding the cwDest mutex, getDest for the same destination
// deadlocks because it also needs the mutex.
func TestPublishDoesNotBlockGetDest(t *testing.T) {
	bq := &blockingQueue{eventsCh: make(chan logs.LogEvent, 1)}
	bq.eventsCh <- &stubLogEvent{} // fill the channel

	target := pusher.Target{Group: "test-group", Stream: "test-stream", Retention: -1}
	dest := &cwDest{
		pusher:   &pusher.Pusher{Target: target, Queue: bq},
		refCount: 1,
	}

	c := &CloudWatchLogs{
		Log:       testutil.Logger{Name: "test"},
		AccessKey: "access_key",
		SecretKey: "secret_key",
		cwDests:   sync.Map{},
	}
	c.cwDests.Store(target, dest)

	// Publish blocks on AddEvent while holding the lock
	go func() {
		dest.Publish([]logs.LogEvent{&stubLogEvent{}})
	}()
	time.Sleep(50 * time.Millisecond)

	// getDest must not deadlock
	done := make(chan struct{})
	go func() {
		c.getDest(target, nil)
		close(done)
	}()

	select {
	case <-done:
		// no deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("DEADLOCK: getDest blocked for 2s waiting for lock held by Publish")
	}
}

type stubLogEvent struct{}

func (s *stubLogEvent) Message() string { return "test" }
func (s *stubLogEvent) Time() time.Time { return time.Now() }
func (s *stubLogEvent) Done()           {}
