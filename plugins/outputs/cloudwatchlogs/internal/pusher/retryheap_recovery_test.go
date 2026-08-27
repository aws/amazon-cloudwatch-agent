// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

// TestRecoveryWhenPermissionGrantedDuringRetry validates that when PLE permissions
// are missing initially but granted while retry is ongoing, the system recovers
// and successfully publishes logs.
func TestRecoveryWhenPermissionGrantedDuringRetry(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()

	// Mock service that initially returns AccessDenied, then succeeds
	mockService := &mockLogsService{}
	accessDeniedErr := &cloudwatchlogs.AccessDeniedException{
		Message_: aws.String("Access denied"),
	}

	// First call fails with AccessDenied
	mockService.On("PutLogEvents", mock.Anything).Return((*cloudwatchlogs.PutLogEventsOutput)(nil), accessDeniedErr).Once()
	// Second call succeeds (permission granted)
	mockService.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Once()

	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("EnsureTargetExists", mock.Anything).Return(nil)

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	// Create batch and track circuit breaker state
	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.events = []*cloudwatchlogs.InputLogEvent{
		{Message: aws.String("test message"), Timestamp: aws.Int64(time.Now().Unix() * 1000)},
	}

	var haltCalled, resumeCalled bool
	var mu sync.Mutex

	// Register circuit breaker callbacks
	batch.addFailCallback(func() {
		mu.Lock()
		haltCalled = true
		mu.Unlock()
	})
	batch.addDoneCallback(func() {
		mu.Lock()
		resumeCalled = true
		mu.Unlock()
	})

	// Set batch ready for immediate retry
	batch.nextRetryTime = time.Now().Add(-1 * time.Second)

	// Push batch to heap
	err := heap.Push(batch)
	assert.NoError(t, err)

	// Process first attempt - should fail with AccessDenied
	processor.processReadyMessages()

	// Wait for async processing to complete
	time.Sleep(100 * time.Millisecond)

	// Verify circuit breaker halted
	mu.Lock()
	assert.True(t, haltCalled, "Circuit breaker should halt on failure")
	assert.False(t, resumeCalled, "Circuit breaker should not resume yet")
	mu.Unlock()

	// Batch should be back in heap for retry
	assert.Equal(t, 1, heap.Size(), "Failed batch should be in retry heap")

	// Simulate permission being granted by waiting for retry time
	// Set batch ready for immediate retry
	batch.nextRetryTime = time.Now().Add(-1 * time.Second)

	// Process second attempt - should succeed
	processor.processReadyMessages()

	// Wait for async processing to complete
	time.Sleep(100 * time.Millisecond)

	// Verify circuit breaker resumed
	mu.Lock()
	assert.True(t, resumeCalled, "Circuit breaker should resume on success")
	mu.Unlock()

	// Heap should be empty (batch successfully sent)
	assert.Equal(t, 0, heap.Size(), "Heap should be empty after successful retry")

	// Verify both PutLogEvents calls were made
	mockService.AssertExpectations(t)
}

// A panic while processing the retry heap must not kill the
// processing goroutine. An expired batch whose drop() path panics (via a state callback) is
// popped and finalized inside processReadyMessages; without the deferred recover the panic
// unwinds processLoop and permanently strands retries for every target.
func TestProcessReadyMessagesRecoversFromPanic(t *testing.T) {
	logger := &testutil.Logger{}
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
	logger := &testutil.Logger{}
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
