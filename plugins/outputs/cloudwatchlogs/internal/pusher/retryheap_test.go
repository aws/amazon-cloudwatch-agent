// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"testing"
	"time"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRetryHeap(t *testing.T) {
	heap := NewRetryHeap(10)
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
	heap := NewRetryHeap(10)
	defer heap.Stop()

	target := Target{Group: "group", Stream: "stream"}
	now := time.Now()

	// Create batches with different retry times (not in order)
	batch1 := newLogEventBatch(target, nil)
	batch1.nextRetryTime = now.Add(3 * time.Second)

	batch2 := newLogEventBatch(target, nil)
	batch2.nextRetryTime = now.Add(1 * time.Second)

	batch3 := newLogEventBatch(target, nil)
	batch3.nextRetryTime = now.Add(2 * time.Second)

	// Push in random order
	heap.Push(batch1)
	heap.Push(batch2)
	heap.Push(batch3)

	// Wait for all to be ready
	time.Sleep(4 * time.Second)

	// Pop ready batches - should come out in order
	ready := heap.PopReady()
	assert.Len(t, ready, 3)
	assert.True(t, ready[0].nextRetryTime.Before(ready[1].nextRetryTime))
	assert.True(t, ready[1].nextRetryTime.Before(ready[2].nextRetryTime))
}

func TestRetryHeapProcessor(t *testing.T) {
	heap := NewRetryHeap(10)
	defer heap.Stop()

	// Create mock components
	mockWorkerPool := NewWorkerPool(2)
	defer mockWorkerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, mockWorkerPool, mockService, mockTargetManager, &testutil.Logger{}, time.Hour)
	defer processor.Stop()

	// Test start/stop
	processor.Start()
	assert.NotNil(t, processor.ticker)

	processor.Stop()
	assert.True(t, processor.stopped)
}

func TestRetryHeapProcessorExpiredBatch(t *testing.T) {
	heap := NewRetryHeap(10)
	defer heap.Stop()

	mockWorkerPool := NewWorkerPool(2)
	defer mockWorkerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, mockWorkerPool, mockService, mockTargetManager, &testutil.Logger{}, 1*time.Millisecond) // Very short expiry

	// Create expired batch
	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.startTime = time.Now().Add(-1 * time.Hour)       // Old start time
	batch.nextRetryTime = time.Now().Add(-1 * time.Second) // Ready now

	heap.Push(batch)

	// Process should drop expired batch
	processor.processReadyMessages()
	assert.Equal(t, 0, heap.Size()) // Expired batch should be removed
}

func TestRetryHeapProcessorSendsBatch(t *testing.T) {
	heap := NewRetryHeap(10)
	defer heap.Stop()

	mockWorkerPool := NewWorkerPool(2)
	defer mockWorkerPool.Stop()
	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}

	processor := NewRetryHeapProcessor(heap, mockWorkerPool, mockService, mockTargetManager, &testutil.Logger{}, time.Hour)

	// Create ready batch
	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.nextRetryTime = time.Now().Add(-1 * time.Second) // Ready now

	heap.Push(batch)

	// Process should send batch
	processor.processReadyMessages()
	assert.Equal(t, 0, heap.Size()) // Batch should be removed from heap
}

func TestRetryHeap_SemaphoreBlockingAndUnblocking(t *testing.T) {
	heap := NewRetryHeap(2) // maxSize = 2
	defer heap.Stop()

	// Fill heap to capacity with batches that will be ready in 3 seconds
	target := Target{Group: "group", Stream: "stream"}
	batch1 := newLogEventBatch(target, nil)
	batch1.nextRetryTime = time.Now().Add(3 * time.Second)
	batch2 := newLogEventBatch(target, nil)
	batch2.nextRetryTime = time.Now().Add(3 * time.Second)

	heap.Push(batch1)
	heap.Push(batch2)

	// Verify heap is at capacity
	if heap.Size() != 2 {
		t.Fatalf("Expected size 2, got %d", heap.Size())
	}

	// Try to push third item - should block
	var pushCompleted bool

	go func() {
		batch3 := newLogEventBatch(target, nil)
		batch3.nextRetryTime = time.Now().Add(time.Hour) // Future time, won't be popped
		heap.Push(batch3)                                // This should block
		pushCompleted = true
	}()

	// Give goroutine time to hit the semaphore block
	time.Sleep(100 * time.Millisecond)

	if pushCompleted {
		t.Fatal("Push should be blocked by semaphore")
	}

	// Wait for batches to become ready, then pop to release semaphore
	time.Sleep(4 * time.Second)
	heap.PopReady()

	// Give time for push to unblock
	time.Sleep(100 * time.Millisecond)

	if !pushCompleted {
		t.Fatal("Push should be unblocked after PopReady")
	}

	// Verify final state - should have 1 item (2 popped, 1 pushed)
	if heap.Size() != 1 {
		t.Fatalf("Expected size 1 after pop/push cycle, got %d", heap.Size())
	}
}
