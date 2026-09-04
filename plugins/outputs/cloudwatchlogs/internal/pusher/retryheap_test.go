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

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

func TestRetryHeap(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Close()

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
	defer heap.Close()

	target := Target{Group: "group", Stream: "stream"}
	now := time.Now()

	// Create batches with distinct retry times already in the past (not in order), so all
	// are ready immediately and PopReady must return them ordered by nextRetryTime.
	batch1 := newLogEventBatch(target, nil)
	batch1.nextRetryTime = now.Add(-10 * time.Millisecond)

	batch2 := newLogEventBatch(target, nil)
	batch2.nextRetryTime = now.Add(-30 * time.Millisecond)

	batch3 := newLogEventBatch(target, nil)
	batch3.nextRetryTime = now.Add(-20 * time.Millisecond)

	// Push in random order
	heap.Push(batch1)
	heap.Push(batch2)
	heap.Push(batch3)

	// Pop ready batches - should come out in order
	ready := heap.PopReady()
	assert.Len(t, ready, 3)
	assert.True(t, ready[0].nextRetryTime.Before(ready[1].nextRetryTime))
	assert.True(t, ready[1].nextRetryTime.Before(ready[2].nextRetryTime))
}

func TestRetryHeapProcessor(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Close()

	// Create mock components with proper signature
	workerPool := NewWorkerPool(2, &testutil.Logger{})
	defer workerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{})
	defer processor.Stop()

	// Test start/stop
	processor.Start()
	processor.Stop()
	assert.True(t, processor.stopped.Load())
}

func TestRetryHeapProcessorExpiredBatch(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Close()

	workerPool := NewWorkerPool(2, &testutil.Logger{})
	defer workerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{})

	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.append(newLogEvent(time.Now(), "test message", nil))
	batch.initializeStartTime()
	batch.expireAfter = time.Now().Add(-1 * time.Hour) // Already expired
	batch.updateRetryMetadata(&cloudwatchlogs.ServiceUnavailableException{})
	batch.nextRetryTime = time.Now().Add(-1 * time.Second)

	var doneCalled, resumeCalled bool
	batch.addDoneCallback(func() { doneCalled = true })
	batch.addResumeCallback(func() { resumeCalled = true })

	heap.Push(batch)
	assert.Equal(t, 1, heap.Size(), "batch should be queued before processing")

	processor.processReadyMessages()
	assert.Equal(t, 0, heap.Size(), "Expired batch should be removed from heap")
	assert.True(t, resumeCalled, "expired batch should resume the circuit breaker to unblock the target")
	assert.False(t, doneCalled, "expired batch was never delivered, so it must not signal success")
}

func TestRetryHeapProcessorSendsBatch(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Close()

	workerPool := NewWorkerPool(2, &testutil.Logger{})
	defer workerPool.Stop()

	mockService := &mockLogsService{}
	mockService.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil)
	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("InitTarget", mock.Anything).Return(nil)

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{})

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
	defer heap.Close()

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
	defer heap.Close()

	workerPool := NewWorkerPool(2, &testutil.Logger{})
	defer workerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{})

	// Process with empty heap - should not panic
	processor.processReadyMessages()

	assert.Equal(t, 0, heap.Size())
}

func TestRetryHeapProcessorFailedBatchGoesBackToHeap(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Close()

	workerPool := NewWorkerPool(2, &testutil.Logger{})
	defer workerPool.Stop()

	// Create failing service with AWS error that triggers retry
	mockService := &mockLogsService{}
	mockService.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, &cloudwatchlogs.ServiceUnavailableException{})

	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("InitTarget", mock.Anything).Return(nil)

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{})

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
	rh.Close()
	rh.Close()

	// After stopping, Push should drop the batch silently
	target := Target{Group: "test-group", Stream: "test-stream"}
	batch := newLogEventBatch(target, nil)

	rh.Push(batch)

	// Verify heap is empty (nothing was pushed)
	assert.Equal(t, 0, rh.Size())
}

func TestRetryHeapProcessorStoppedProcessReadyMessages(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Close()

	workerPool := NewWorkerPool(2, &testutil.Logger{})
	defer workerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{})

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

// Stop() must not hang when its own flush blocks in workerPool.Submit on a saturated pool.
// Stop bounds the flush with stopFlushTimeout and the process-loop wait with stopWaitTimeout,
// so shutdown proceeds even while a tick is mid-flush.
//
// The window is made deterministic here: Stop()'s flush blocks in workerPool.Submit until
// the test releases the sends.
func TestRetryHeapProcessorStopDoesNotDeadlock(t *testing.T) {
	logger := &testutil.Logger{}

	release := make(chan struct{})
	var inFlight atomic.Int32
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		inFlight.Add(1)
		<-release
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})

	workerPool := NewWorkerPool(1, &testutil.Logger{}) // 1 worker + task buffer 2 -> Submit blocks on the 4th batch
	retryHeap := NewRetryHeap(logger)

	// Saturate the pool first: the processor's own ticker drains these and parks on release.
	for i := 0; i < 3; i++ {
		require.NoError(t, retryHeap.Push(readyBatch("g", nil, nil)))
	}

	p := NewRetryHeapProcessor(retryHeap, workerPool, service, NewTargetManager(logger, service), logger)
	p.Start()
	require.Eventually(t, func() bool { return inFlight.Load() > 0 }, 5*time.Second, 10*time.Millisecond,
		"test setup: expected the worker pool to be saturated")

	// Give Stop()'s own flush something to do, so it is the caller that blocks in Submit.
	for i := 0; i < 5; i++ {
		require.NoError(t, retryHeap.Push(readyBatch("g", nil, nil)))
	}

	stopped := make(chan struct{})
	go func() { p.Stop(); close(stopped) }()

	// Stop()'s flush is now blocked inside flushReadyBatches -> Submit; let the 100ms ticker
	// fire concurrently to confirm neither path wedges shutdown.
	time.Sleep(400 * time.Millisecond)
	close(release)

	select {
	case <-stopped:
	case <-time.After(15 * time.Second):
		t.Fatal("Stop() deadlocked: flush or process-loop wait was not bounded on a saturated pool")
	}
	workerPool.Stop()
}

// Stop() must be idempotent and safe under concurrent callers: it uses sync.Once, so a
// regression to a plain flag would double-close stopCh and panic.
func TestProcessorStopIsIdempotentAndConcurrencySafe(t *testing.T) {
	logger := &testutil.Logger{}
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})
	workerPool := NewWorkerPool(2, &testutil.Logger{})
	defer workerPool.Stop()
	retryHeap := NewRetryHeap(logger)

	p := NewRetryHeapProcessor(retryHeap, workerPool, service, NewTargetManager(logger, service), logger)
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

// Pushing to a closed heap must fail cleanly so the caller can abandon rather than block or
// panic -- the path sender.Send() takes during shutdown.
func TestHeapPushAfterStopFailsCleanly(t *testing.T) {
	logger := &testutil.Logger{}
	h := NewRetryHeap(logger)
	require.NoError(t, h.Push(readyBatch("g", nil, nil)))
	h.Close()

	err := h.Push(readyBatch("g", nil, nil))
	require.Error(t, err, "push after Stop must report failure, not silently succeed")
	assert.Contains(t, err.Error(), "stopped")
	h.Close() // idempotent
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

	pool := NewWorkerPool(1, &testutil.Logger{}) // tasks buffer = size*2 = 2
	defer pool.Stop()

	// Occupy the single worker and fill the task buffer so any further Submit blocks.
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	pool.Submit(func() { defer wg.Done(); <-release })
	pool.Submit(func() { <-release })
	pool.Submit(func() { <-release })

	retryHeap := NewRetryHeap(logger)
	defer retryHeap.Close()
	p := NewRetryHeapProcessor(retryHeap, pool, svc, NewTargetManager(logger, svc), logger)

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

	pool := NewWorkerPool(1, &testutil.Logger{}) // task buffer = 2
	defer pool.Stop()
	release := make(chan struct{})
	pool.Submit(func() { <-release }) // occupy the worker
	pool.Submit(func() { <-release }) // fill buffer
	pool.Submit(func() { <-release }) // fill buffer

	heap := NewRetryHeap(logger)
	defer heap.Close()
	p := NewRetryHeapProcessor(heap, pool, svc, NewTargetManager(logger, svc), logger)
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

// A batch dropped because the retry heap is already stopped (shutdown in
// progress) must not fire done callbacks. done() runs updateState() AND every
// doneCallback -- per-event LogEvent.Done() plus the queue's onSuccessCallback -- which
// marks never-delivered events as delivered and advances file offsets past them, so the
// events are neither shipped nor re-read after restart.
func TestHeapStoppedDropDoesNotSignalSuccess(t *testing.T) {
	logger := &testutil.Logger{}
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	})

	retryHeap := NewRetryHeap(logger)
	retryHeap.Close() // shutdown already closed the heap

	var delivered, offsetsAdvanced atomic.Int32
	batch := readyBatch("g", func() { delivered.Add(1) }, func() { offsetsAdvanced.Add(1) })

	newSender(logger, service, NewTargetManager(logger, service), retryHeap).Send(batch)

	assert.Zero(t, delivered.Load(),
		"undelivered batch dropped at shutdown fired done callbacks: events are reported delivered")
	assert.Zero(t, offsetsAdvanced.Load(),
		"undelivered batch dropped at shutdown advanced file offsets: events are lost across restart")
}

// Second site: the same false-success signal on the expired-batch drop
// inside flushReadyBatches. Expiry is a permanent give-up, so offsets SHOULD advance --
// but done callbacks must not fire, because nothing was delivered.
func TestExpiredBatchDropDoesNotSignalSuccess(t *testing.T) {
	logger := &testutil.Logger{}
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})

	workerPool := NewWorkerPool(1, &testutil.Logger{})
	defer workerPool.Stop()
	retryHeap := NewRetryHeap(logger)

	var delivered, offsetsAdvanced atomic.Int32
	batch := readyBatch("g", func() { delivered.Add(1) }, func() { offsetsAdvanced.Add(1) })
	batch.expireAfter = time.Now().Add(-time.Hour) // already past its expiry window
	require.NoError(t, retryHeap.Push(batch))

	p := NewRetryHeapProcessor(retryHeap, workerPool, service, NewTargetManager(logger, service), logger)
	p.flushReadyBatches()

	require.True(t, batch.isExpired(), "test setup: batch must be expired to exercise the drop path")
	assert.Zero(t, delivered.Load(),
		"expired batch fired done callbacks: never-delivered events are reported delivered")
	assert.Equal(t, int32(1), offsetsAdvanced.Load(),
		"expired batch is a permanent give-up, so offsets should advance exactly once")
}

// TestRetryHeapSuccessCallsStateCallback verifies that when a batch succeeds
// on retry through the heap, state callbacks fire to persist file offsets.
func TestRetryHeapSuccessCallsStateCallback(t *testing.T) {
	logger := &testutil.Logger{}
	target := Target{Group: "group", Stream: "stream"}

	queue := &mockFileRangeQueue{}
	queue.On("ID").Return("file1")
	queue.On("Enqueue", mock.Anything).Return()

	service := &stubLogsService{
		ple: func(_ *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
		cls: func(_ *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		},
		clg: func(_ *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
			return &cloudwatchlogs.CreateLogGroupOutput{}, nil
		},
		dlg: func(_ *cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
		},
	}

	retryHeap := NewRetryHeap(logger)
	workerPool := NewWorkerPool(2, &testutil.Logger{})
	tm := NewTargetManager(logger, service)
	defer retryHeap.Close()
	defer workerPool.Stop()

	processor := NewRetryHeapProcessor(retryHeap, workerPool, service, tm, logger)

	batch := newStatefulBatch(target, queue)
	batch.nextRetryTime = time.Now().Add(-1 * time.Second)

	err := retryHeap.Push(batch)
	assert.NoError(t, err)

	processor.processReadyMessages()
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, 0, retryHeap.Size(), "Heap should be empty after success")
	queue.AssertCalled(t, "Enqueue", mock.Anything)
}

// TestRetryHeapExpiryCallsStateCallback verifies that when a batch expires
// after 14 days without successfully publishing, state callbacks still fire
// to persist file offsets and prevent re-reading on restart.
func TestRetryHeapExpiryCallsStateCallback(t *testing.T) {
	logger := &testutil.Logger{}
	target := Target{Group: "group", Stream: "stream"}

	queue := &mockFileRangeQueue{}
	queue.On("ID").Return("file1")
	queue.On("Enqueue", mock.Anything).Return()

	service := &stubLogsService{
		ple: func(_ *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
			return nil, &cloudwatchlogs.ServiceUnavailableException{}
		},
		cls: func(_ *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		},
		clg: func(_ *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
			return &cloudwatchlogs.CreateLogGroupOutput{}, nil
		},
		dlg: func(_ *cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
		},
	}

	retryHeap := NewRetryHeap(logger)
	workerPool := NewWorkerPool(2, &testutil.Logger{})
	tm := NewTargetManager(logger, service)
	defer retryHeap.Close()
	defer workerPool.Stop()

	processor := NewRetryHeapProcessor(retryHeap, workerPool, service, tm, logger)

	batch := newStatefulBatch(target, queue)
	batch.initializeStartTime()
	batch.expireAfter = time.Now().Add(-10 * time.Millisecond) // Already expired
	batch.updateRetryMetadata(&cloudwatchlogs.ServiceUnavailableException{})
	batch.nextRetryTime = time.Now().Add(-1 * time.Second) // Override to make it ready

	err := retryHeap.Push(batch)
	assert.NoError(t, err)

	processor.processReadyMessages()
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, 0, retryHeap.Size(), "Expired batch should be removed")
	queue.AssertCalled(t, "Enqueue", mock.Anything)
}

// TestShutdownDoesNotCallStateCallback verifies that during a clean shutdown
// via Stop(), remaining batches in the retry heap do NOT have their state
// callbacks invoked. This prevents marking undelivered data as processed.
func TestShutdownDoesNotCallStateCallback(t *testing.T) {
	logger := &testutil.Logger{}
	target := Target{Group: "group", Stream: "stream"}

	var stateCallCount atomic.Int32

	retryHeap := NewRetryHeap(logger)
	workerPool := NewWorkerPool(2, &testutil.Logger{})
	defer workerPool.Stop()

	service := &stubLogsService{
		ple: func(_ *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
			return nil, &cloudwatchlogs.ServiceUnavailableException{}
		},
		cls: func(_ *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		},
		clg: func(_ *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
			return &cloudwatchlogs.CreateLogGroupOutput{}, nil
		},
		dlg: func(_ *cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
		},
	}
	tm := NewTargetManager(logger, service)

	processor := NewRetryHeapProcessor(retryHeap, workerPool, service, tm, logger)
	processor.Start()

	// Push a batch with a future retry time so it won't be processed before Stop
	batch := newLogEventBatch(target, nil)
	batch.append(newLogEvent(time.Now(), "test", nil))
	batch.addStateCallback(func() { stateCallCount.Add(1) })
	batch.nextRetryTime = time.Now().Add(1 * time.Hour) // Not ready yet

	err := retryHeap.Push(batch)
	assert.NoError(t, err)

	// Stop the processor — batch is still in heap, not ready
	processor.Stop()
	retryHeap.Close()

	assert.Equal(t, int32(0), stateCallCount.Load(),
		"State callback should not be called for unprocessed batches during shutdown")
	assert.Equal(t, 1, retryHeap.Size(), "Batch should remain in heap after shutdown")
}
