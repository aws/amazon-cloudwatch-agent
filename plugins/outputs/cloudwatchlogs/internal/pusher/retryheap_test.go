// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"testing"
	"time"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

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

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, time.Hour, retryer.NewLogThrottleRetryer(&testutil.Logger{}))
	defer processor.Stop()

	// Test start/stop
	processor.Start()
	processor.Stop()
	assert.True(t, processor.stopped)
}

func TestRetryHeapProcessorExpiredBatch(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, 1*time.Millisecond, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	// Create expired batch
	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.startTime = time.Now().Add(-1 * time.Hour)
	batch.nextRetryTime = time.Now().Add(-1 * time.Second)

	heap.Push(batch)

	// Process should drop expired batch
	processor.processReadyMessages()
	assert.Equal(t, 0, heap.Size())
}

func TestRetryHeapProcessorSendsBatch(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, time.Hour, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	// Create ready batch (retryTime already past)
	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.nextRetryTime = time.Now().Add(-1 * time.Second)

	heap.Push(batch)

	// Process should send batch
	processor.processReadyMessages()
	assert.Equal(t, 0, heap.Size())
}

func TestRetryHeap_UnboundedPush(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{}) // maxSize parameter ignored (unbounded)
	defer heap.Stop()

	// Push multiple batches without blocking
	target := Target{Group: "group", Stream: "stream"}
	batch1 := newLogEventBatch(target, nil)
	batch1.nextRetryTime = time.Now().Add(3 * time.Second)
	batch2 := newLogEventBatch(target, nil)
	batch2.nextRetryTime = time.Now().Add(3 * time.Second)
	batch3 := newLogEventBatch(target, nil)
	batch3.nextRetryTime = time.Now().Add(3 * time.Second)

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

	time.Sleep(3 * time.Second)

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

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, time.Hour, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

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

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, time.Hour, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

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

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, time.Hour, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

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
	assert.True(t, processor.stopped)
}
