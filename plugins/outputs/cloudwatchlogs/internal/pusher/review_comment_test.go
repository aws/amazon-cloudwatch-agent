// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

// Review comment (batch.go): drop() must track undelivered events via an atomic counter so
// data loss is observable. Every event in a dropped batch counts.
func TestDropIncrementsDroppedLogEvents(t *testing.T) {
	before := DroppedLogEvents()

	b := newLogEventBatch(Target{Group: "g", Stream: "s"}, nil)
	b.append(newLogEvent(time.Now(), "a", func() {}))
	b.append(newLogEvent(time.Now(), "b", func() {}))
	b.drop()

	require.Equal(t, before+2, DroppedLogEvents(),
		"drop() must add every undelivered event in the batch to the dropped counter")
}

// Review comment (retryheap.go): a panic while processing the retry heap must not kill the
// processing goroutine. An expired batch whose drop() path panics (via a state callback) is
// popped and finalized inside processReadyMessages; without the deferred recover the panic
// unwinds processLoop and permanently strands retries for every target.
func TestProcessReadyMessagesRecoversFromPanic(t *testing.T) {
	logger := testutil.NewNopLogger()
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})
	workerPool := NewWorkerPool(1)
	defer workerPool.Stop()
	retryHeap := NewRetryHeap(logger)
	defer retryHeap.Stop()
	p := NewRetryHeapProcessor(retryHeap, workerPool, service, NewTargetManager(logger, service), logger, nil)

	b := readyBatch("g", nil, func() { panic("boom in drop path") })
	b.expireAfter = time.Now().Add(-time.Hour) // force the expired -> drop() path
	require.NoError(t, retryHeap.Push(b))

	require.NotPanics(t, func() { p.processReadyMessages() },
		"a panic in one batch must be recovered so the retry loop survives for other targets")
	require.Zero(t, retryHeap.Size(), "the panicking batch should still have been popped from the heap")
}

// TestFlushReadyBatchesPanicDoesNotStrandLaterBatches covers what the single-batch test
// above cannot: PopReady removes EVERY ready batch from the heap before the loop runs, so a
// recover scoped to the whole loop would silently lose the batches queued behind a panicking
// one -- they are already off the heap and never sent, re-pushed, or finalized.
func TestFlushReadyBatchesPanicDoesNotStrandLaterBatches(t *testing.T) {
	logger := testutil.NewNopLogger()
	var sent atomic.Int32
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		sent.Add(1)
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})
	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()
	retryHeap := NewRetryHeap(logger)
	defer retryHeap.Stop()
	p := NewRetryHeapProcessor(retryHeap, workerPool, service, NewTargetManager(logger, service), logger, nil)

	// First batch panics during its expired -> drop() path.
	bad := readyBatch("bad", nil, func() { panic("boom in drop path") })
	bad.expireAfter = time.Now().Add(-time.Hour)
	bad.nextRetryTime = time.Now().Add(-time.Minute) // pop first
	require.NoError(t, retryHeap.Push(bad))

	// Two healthy batches behind it must still be submitted.
	const healthy = 2
	for i := 0; i < healthy; i++ {
		g := readyBatch("good", nil, nil)
		g.nextRetryTime = time.Now().Add(-time.Second)
		require.NoError(t, retryHeap.Push(g))
	}

	require.NotPanics(t, func() { p.processReadyMessages() })
	require.Zero(t, retryHeap.Size(), "all ready batches should have been popped")

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && int(sent.Load()) < healthy {
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(t, healthy, int(sent.Load()),
		"batches queued behind a panicking batch must still be sent, not silently dropped")
}
