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

	// Wait for pushToHeapWorker to process
	time.Sleep(10 * time.Millisecond)
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

	// Create mock senderPool
	mockSenderPool := &mockSenderPool{}
	processor := NewRetryHeapProcessor(heap, mockSenderPool, &testutil.Logger{}, time.Hour)
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

	mockSenderPool := &mockSenderPool{}
	processor := NewRetryHeapProcessor(heap, mockSenderPool, &testutil.Logger{}, 1*time.Millisecond) // Very short expiry

	// Create expired batch
	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.startTime = time.Now().Add(-1 * time.Hour)       // Old start time
	batch.nextRetryTime = time.Now().Add(-1 * time.Second) // Ready now

	heap.Push(batch)
	time.Sleep(10 * time.Millisecond) // Wait for pushToHeapWorker

	// Process should drop expired batch
	processor.processReadyMessages()
	assert.Equal(t, 0, heap.Size())
	assert.Equal(t, 0, mockSenderPool.sendCount) // Should not send expired batch
}

func TestRetryHeapProcessorSendsBatch(t *testing.T) {
	heap := NewRetryHeap(10)
	defer heap.Stop()

	mockSenderPool := &mockSenderPool{}
	processor := NewRetryHeapProcessor(heap, mockSenderPool, &testutil.Logger{}, time.Hour)

	// Create ready batch
	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.nextRetryTime = time.Now().Add(-1 * time.Second) // Ready now

	heap.Push(batch)
	time.Sleep(10 * time.Millisecond) // Wait for pushToHeapWorker

	// Process should send batch
	processor.processReadyMessages()
	assert.Equal(t, 0, heap.Size())
	assert.Equal(t, 1, mockSenderPool.sendCount)
}

// Mock senderPool for testing
type mockSenderPool struct {
	sendCount int
}

func (m *mockSenderPool) Send(_ *logEventBatch) {
	m.sendCount++
}

func (m *mockSenderPool) Stop()                          {}
func (m *mockSenderPool) SetRetryDuration(time.Duration) {}
func (m *mockSenderPool) RetryDuration() time.Duration   { return time.Hour }
