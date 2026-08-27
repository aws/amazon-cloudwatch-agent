// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

func TestRetryHeap(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	// Test empty heap
	assert.Equal(t, 0, heap.Size())
	ready := heap.PopReady()
	assert.Empty(t, ready)

	// Create test batches
	target := Target{Group: "group", Stream: "stream"}
	batch1 := newLogEventBatch(target, nil)
	batch1.nextRetryTime = time.Now().Add(1 * time.Second)

	batch2 := newLogEventBatch(target, nil)
	batch2.nextRetryTime = time.Now().Add(-1 * time.Second) // Ready now

	// Push batches
	err := heap.Push(batch1)
	assert.NoError(t, err)
	err = heap.Push(batch2)
	assert.NoError(t, err)

	assert.Equal(t, 2, heap.Size())

	// Pop ready batches
	ready = heap.PopReady()
	assert.Len(t, ready, 1)
	assert.Equal(t, batch2, ready[0])
	assert.Equal(t, 1, heap.Size())
}

func TestRetryHeapOrdering(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	target := Target{Group: "group", Stream: "stream"}
	now := time.Now()

	// Create batches with different retry times (not in order)
	batch1 := newLogEventBatch(target, nil)
	batch1.nextRetryTime = now.Add(30 * time.Millisecond)

	batch2 := newLogEventBatch(target, nil)
	batch2.nextRetryTime = now.Add(10 * time.Millisecond)

	batch3 := newLogEventBatch(target, nil)
	batch3.nextRetryTime = now.Add(20 * time.Millisecond)

	// Push in random order
	heap.Push(batch1)
	heap.Push(batch2)
	heap.Push(batch3)

	// Wait for all to be ready
	time.Sleep(100 * time.Millisecond)

	// Pop ready batches - should come out in order
	ready := heap.PopReady()
	assert.Len(t, ready, 3)
	assert.True(t, ready[0].nextRetryTime.Before(ready[1].nextRetryTime))
	assert.True(t, ready[1].nextRetryTime.Before(ready[2].nextRetryTime))
}

func TestRetryHeapProcessor(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	// Create mock components with proper signature
	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, retryer.NewLogThrottleRetryer(&testutil.Logger{}))
	defer processor.Stop()

	// Test start/stop
	processor.Start()
	processor.Stop()
	assert.True(t, processor.stopped.Load())
}

func TestRetryHeapProcessorExpiredBatch(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.initializeStartTime()
	batch.expireAfter = time.Now().Add(-1 * time.Hour) // Already expired
	batch.nextRetryTime = time.Now().Add(-1 * time.Second)

	var doneCalled, resumeCalled bool
	batch.addDoneCallback(func() { doneCalled = true })
	batch.addResumeCallback(func() { resumeCalled = true })

	heap.Push(batch)

	processor.processReadyMessages()
	assert.Equal(t, 0, heap.Size(), "Expired batch should be removed from heap")
	assert.True(t, resumeCalled, "expired batch should resume the circuit breaker")
	assert.False(t, doneCalled, "expired batch was never delivered, so it must not signal success")
}

func TestRetryHeapProcessorSendsBatch(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()

	mockService := &mockLogsService{}
	mockService.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil)
	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("InitTarget", mock.Anything).Return(nil)

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.append(newLogEvent(time.Now(), "test message", nil))
	batch.nextRetryTime = time.Now().Add(-1 * time.Second)

	var doneCalled atomic.Bool
	batch.addDoneCallback(func() { doneCalled.Store(true) })

	heap.Push(batch)

	processor.processReadyMessages()
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, 0, heap.Size())
	assert.True(t, doneCalled.Load(), "Batch done callback should be called on successful send")
	mockService.AssertCalled(t, "PutLogEvents", mock.Anything)
}

func TestRetryHeap_UnboundedPush(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{}) // maxSize parameter ignored (unbounded)
	defer heap.Stop()

	// Push multiple batches without blocking
	target := Target{Group: "group", Stream: "stream"}
	batch1 := newLogEventBatch(target, nil)
	batch1.nextRetryTime = time.Now().Add(50 * time.Millisecond)
	batch2 := newLogEventBatch(target, nil)
	batch2.nextRetryTime = time.Now().Add(50 * time.Millisecond)
	batch3 := newLogEventBatch(target, nil)
	batch3.nextRetryTime = time.Now().Add(50 * time.Millisecond)

	// All pushes should succeed immediately (non-blocking)
	err := heap.Push(batch1)
	assert.NoError(t, err)
	err = heap.Push(batch2)
	assert.NoError(t, err)
	err = heap.Push(batch3)
	assert.NoError(t, err)

	// Verify heap can grow beyond original maxSize parameter
	if heap.Size() != 3 {
		t.Fatalf("Expected size 3, got %d", heap.Size())
	}

	time.Sleep(100 * time.Millisecond)

	// Pop ready batches
	readyBatches := heap.PopReady()
	assert.Len(t, readyBatches, 3, "Should pop exactly 3 ready batches")

	for _, batch := range readyBatches {
		assert.Equal(t, "group", batch.Group)
		assert.Equal(t, "stream", batch.Stream)
	}

	// Verify heap is empty
	if heap.Size() != 0 {
		t.Fatalf("Expected size 0 after pop, got %d", heap.Size())
	}
}

func TestRetryHeapProcessorNoReadyBatches(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	// Process with empty heap - should not panic
	processor.processReadyMessages()

	assert.Equal(t, 0, heap.Size())
}

func TestRetryHeapProcessorFailedBatchGoesBackToHeap(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()

	// Create failing service with AWS error that triggers retry
	mockService := &mockLogsService{}
	mockService.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, &cloudwatchlogs.ServiceUnavailableException{})

	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("InitTarget", mock.Anything).Return(nil)

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	processor.Start()
	defer processor.Stop()

	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.nextRetryTime = time.Now().Add(-1 * time.Second)

	timestamp := time.Now().UnixMilli()
	message := "test message"
	batch.events = append(batch.events, &cloudwatchlogs.InputLogEvent{
		Message:   &message,
		Timestamp: &timestamp,
	})

	heap.Push(batch)

	// Wait for goroutine to process the batch
	time.Sleep(200 * time.Millisecond)

	mockService.AssertExpectations(t)
	// Batch should be back in heap after async failure
	assert.Equal(t, 1, heap.Size(), "Failed batch should go back to RetryHeap after async processing")
}

func TestRetryHeapStopTwice(t *testing.T) {
	rh := NewRetryHeap(&testutil.Logger{})

	// Call Stop twice - should not panic
	rh.Stop()
	rh.Stop()

	// After stopping, Push should drop the batch silently
	target := Target{Group: "test-group", Stream: "test-stream"}
	batch := newLogEventBatch(target, nil)

	rh.Push(batch)

	// Verify heap is empty (nothing was pushed)
	assert.Equal(t, 0, rh.Size())
}

func TestRetryHeapProcessorStoppedProcessReadyMessages(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	// Add a ready batch to the heap
	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.nextRetryTime = time.Now().Add(-1 * time.Second) // Ready for retry
	heap.Push(batch)

	// Verify batch is in heap
	assert.Equal(t, 1, heap.Size())

	// Stop the processor (this will process the batch as part of shutdown)
	processor.Stop()

	// Verify the processor processed the batch during shutdown (heap is now empty)
	assert.Equal(t, 0, heap.Size())

	// Add another batch after stopping
	batch2 := newLogEventBatch(target, nil)
	batch2.nextRetryTime = time.Now().Add(-1 * time.Second)
	heap.Push(batch2)
	assert.Equal(t, 1, heap.Size())

	// Calling processReadyMessages on stopped processor should not panic and should not process
	assert.NotPanics(t, func() {
		processor.processReadyMessages()
	})

	// Verify the stopped processor didn't process the new batch
	assert.Equal(t, 1, heap.Size())

	// Verify processor is marked as stopped
	assert.True(t, processor.stopped.Load())
}

// RetryHeapProcessor.Stop() takes stopMu with a deferred Unlock and holds it
// across wg.Wait(). processLoop's ticker path calls processReadyMessages(), which takes
// the same stopMu. If a tick lands while Stop() holds the lock, processLoop never returns
// to its select, never observes stopCh, and never calls wg.Done() -- so Stop() waits
// forever holding the lock processLoop needs.
//
// The window is made deterministic here: Stop() flushes while holding the lock, and the
// flush blocks in workerPool.Submit until the test releases the sends.
func TestRetryHeapProcessorStopDoesNotDeadlock(t *testing.T) {
	logger := &testutil.Logger{}

	release := make(chan struct{})
	var inFlight atomic.Int32
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		inFlight.Add(1)
		<-release
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})

	workerPool := NewWorkerPool(1) // 1 worker + task buffer 2 -> Submit blocks on the 4th batch
	retryHeap := NewRetryHeap(logger)

	// Saturate the pool first: the processor's own ticker drains these and parks on release.
	for i := 0; i < 3; i++ {
		require.NoError(t, retryHeap.Push(readyBatch("g", nil, nil)))
	}

	p := NewRetryHeapProcessor(retryHeap, workerPool, service, NewTargetManager(logger, service), logger, nil)
	p.Start()
	require.Eventually(t, func() bool { return inFlight.Load() > 0 }, 5*time.Second, 10*time.Millisecond,
		"test setup: expected the worker pool to be saturated")

	// Now give Stop()'s own flush something to do, so IT is the caller that blocks in
	// Submit while holding stopMu (the tick path releases stopMu before flushing).
	for i := 0; i < 5; i++ {
		require.NoError(t, retryHeap.Push(readyBatch("g", nil, nil)))
	}

	stopped := make(chan struct{})
	go func() { p.Stop(); close(stopped) }()

	// Stop() is now blocked inside flushReadyBatches -> Submit while holding stopMu.
	// Give the 100ms ticker time to fire and park processLoop on the same lock.
	time.Sleep(400 * time.Millisecond)
	close(release)

	select {
	case <-stopped:
	case <-time.After(15 * time.Second):
		t.Fatal("Stop() deadlocked: stopMu is held across wg.Wait() while processLoop's " +
			"ticker path blocks acquiring the same stopMu")
	}
	workerPool.Stop()
}

// N1: Stop() must be idempotent and safe under concurrent callers. The fix replaced a mutex
// with sync.Once; a regression here would double-close stopCh and panic.
func TestProcessorStopIsIdempotentAndConcurrencySafe(t *testing.T) {
	logger := &testutil.Logger{}
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})
	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()
	retryHeap := NewRetryHeap(logger)

	p := NewRetryHeapProcessor(retryHeap, workerPool, service, NewTargetManager(logger, service), logger, nil)
	p.Start()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); p.Stop() }()
	}
	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()

	select {
	case <-finished:
	case <-time.After(20 * time.Second):
		t.Fatal("concurrent Stop() calls deadlocked")
	}
	assert.True(t, p.stopped.Load(), "processor should be marked stopped")
	p.Stop() // once more, after the fact
}

// N2: pushing to a closed heap must fail cleanly so the caller can abandon rather than block
// or panic. This is the path sender.Send() takes during shutdown.
func TestHeapPushAfterStopFailsCleanly(t *testing.T) {
	logger := &testutil.Logger{}
	h := NewRetryHeap(logger)
	require.NoError(t, h.Push(readyBatch("g", nil, nil)))
	h.Stop()

	err := h.Push(readyBatch("g", nil, nil))
	require.Error(t, err, "push after Stop must report failure, not silently succeed")
	assert.Contains(t, err.Error(), "stopped")
	h.Stop() // idempotent
}

// RetryHeapProcessor.Stop() flushes ready batches through
// senderPool.Send -> workerPool.Submit, which blocks while the pool is saturated. The
// pool is still running at that point (Close stops the processor BEFORE the pool), so
// Submit has no stopCh to fall through and shutdown can hang unbounded.
func TestRetryHeapProcessorStopDoesNotBlockOnSaturatedPool(t *testing.T) {
	logger := &testutil.Logger{}
	svc := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})

	pool := NewWorkerPool(1) // tasks buffer = size*2 = 2
	defer pool.Stop()

	// Occupy the single worker and fill the task buffer so any further Submit blocks.
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	pool.Submit(func() { defer wg.Done(); <-release })
	pool.Submit(func() { <-release })
	pool.Submit(func() { <-release })

	retryHeap := NewRetryHeap(logger)
	defer retryHeap.Stop()
	p := NewRetryHeapProcessor(retryHeap, pool, svc, NewTargetManager(logger, svc), logger, nil)

	// Ready batches force Stop()'s flush to Submit into the saturated pool.
	for i := 0; i < 3; i++ {
		require.NoError(t, retryHeap.Push(readyBatch("g", nil, nil)))
	}

	stopped := make(chan struct{})
	go func() {
		p.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		// Stop returned: shutdown is bounded even with a saturated pool.
	case <-time.After(20 * time.Second):
		close(release)
		wg.Wait()
		t.Fatal("RetryHeapProcessor.Stop() blocked >20s flushing into a saturated worker " +
			"pool; shutdown must be bounded")
	}

	close(release)
	wg.Wait()
}

// Stop must not hang when the periodic processLoop is already parked inside
// workerPool.Submit on a saturated pool. Stop's own flush is time-bounded, but wg.Wait()
// waits for processLoop, and the plugin stops the worker pool only AFTER this returns
// (cloudwatchlogs.go: processor at :124, pool at :128) -- so nothing releases that Submit.
func TestProcessorStopWhenProcessLoopBlockedInSubmit(t *testing.T) {
	logger := &testutil.Logger{}
	svc := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})

	pool := NewWorkerPool(1) // task buffer = 2
	defer pool.Stop()
	release := make(chan struct{})
	pool.Submit(func() { <-release }) // occupy the worker
	pool.Submit(func() { <-release }) // fill buffer
	pool.Submit(func() { <-release }) // fill buffer

	heap := NewRetryHeap(logger)
	defer heap.Stop()
	p := NewRetryHeapProcessor(heap, pool, svc, NewTargetManager(logger, svc), logger, nil)
	require.NoError(t, heap.Push(readyBatch("g", nil, nil)))

	p.Start()
	time.Sleep(500 * time.Millisecond) // let a tick park processLoop inside Submit

	stopped := make(chan struct{})
	go func() { p.Stop(); close(stopped) }()

	select {
	case <-stopped:
	case <-time.After(25 * time.Second):
		close(release)
		t.Fatal("RetryHeapProcessor.Stop() hung: processLoop is parked in workerPool.Submit " +
			"and the pool is only stopped after Stop() returns, so nothing releases it")
	}
	close(release)
}
