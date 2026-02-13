// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

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
		Message_: stringPtr("Access denied"),
	}

	// First call fails with AccessDenied
	mockService.On("PutLogEvents", mock.Anything).Return((*cloudwatchlogs.PutLogEventsOutput)(nil), accessDeniedErr).Once()
	// Second call succeeds (permission granted)
	mockService.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Once()

	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("EnsureTargetExists", mock.Anything).Return(nil)

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, time.Hour, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	// Create batch and track circuit breaker state
	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.events = []*cloudwatchlogs.InputLogEvent{
		{Message: stringPtr("test message"), Timestamp: int64Ptr(time.Now().Unix() * 1000)},
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

// TestRecoveryAfterSystemRestart validates that when the system restarts with
// retry ongoing, it resumes correctly by loading state and continuing retries.
func TestRecoveryAfterSystemRestart(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()

	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("EnsureTargetExists", mock.Anything).Return(nil)

	// Simulate system restart scenario:
	// 1. Initial failure puts batch in retry state
	// 2. System "restarts" (new processor instance)
	// 3. Batch is reloaded with retry metadata intact
	// 4. Retry succeeds

	target := Target{Group: "group", Stream: "stream"}
	batch := newLogEventBatch(target, nil)
	batch.events = []*cloudwatchlogs.InputLogEvent{
		{Message: stringPtr("test message"), Timestamp: int64Ptr(time.Now().Unix() * 1000)},
	}

	// Simulate batch that was in retry state before restart
	batch.retryCountShort = 2
	batch.startTime = time.Now().Add(-5 * time.Minute)
	batch.nextRetryTime = time.Now().Add(-1 * time.Second) // Ready for retry
	batch.lastError = errors.New("previous error before restart")

	var resumeCalled bool
	var mu sync.Mutex

	batch.addDoneCallback(func() {
		mu.Lock()
		resumeCalled = true
		mu.Unlock()
	})

	// Mock successful retry after restart
	mockService.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Once()

	// Create new processor (simulating restart)
	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, time.Hour, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	// Push batch with existing retry metadata
	err := heap.Push(batch)
	assert.NoError(t, err)

	// Process should succeed
	processor.processReadyMessages()

	// Wait for async processing to complete
	time.Sleep(100 * time.Millisecond)

	// Verify circuit breaker resumed
	mu.Lock()
	assert.True(t, resumeCalled, "Circuit breaker should resume after successful retry post-restart")
	mu.Unlock()

	// Heap should be empty
	assert.Equal(t, 0, heap.Size(), "Heap should be empty after successful retry")

	// Verify retry metadata was preserved
	assert.Equal(t, 2, batch.retryCountShort, "Retry count should be preserved across restart")
	assert.False(t, batch.startTime.IsZero(), "Start time should be preserved across restart")

	mockService.AssertExpectations(t)
}

// TestRecoveryWithMultipleTargets validates that when one target has permission
// issues, other healthy targets continue publishing successfully.
func TestRecoveryWithMultipleTargets(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()

	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("EnsureTargetExists", mock.Anything).Return(nil)

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, time.Hour, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	// Create two targets
	target1 := Target{Group: "group1", Stream: "stream1"}
	target2 := Target{Group: "group2", Stream: "stream2"}

	batch1 := newLogEventBatch(target1, nil)
	batch1.events = []*cloudwatchlogs.InputLogEvent{
		{Message: stringPtr("message1"), Timestamp: int64Ptr(time.Now().Unix() * 1000)},
	}
	batch1.nextRetryTime = time.Now().Add(-1 * time.Second)

	batch2 := newLogEventBatch(target2, nil)
	batch2.events = []*cloudwatchlogs.InputLogEvent{
		{Message: stringPtr("message2"), Timestamp: int64Ptr(time.Now().Unix() * 1000)},
	}
	batch2.nextRetryTime = time.Now().Add(-1 * time.Second)

	var halt1Called, resume1Called, resume2Called bool
	var mu sync.Mutex

	// Target 1 fails with AccessDenied
	batch1.addFailCallback(func() {
		mu.Lock()
		halt1Called = true
		mu.Unlock()
	})
	batch1.addDoneCallback(func() {
		mu.Lock()
		resume1Called = true
		mu.Unlock()
	})

	// Target 2 succeeds
	batch2.addDoneCallback(func() {
		mu.Lock()
		resume2Called = true
		mu.Unlock()
	})

	// Mock responses: target1 fails, target2 succeeds
	accessDeniedErr := &cloudwatchlogs.AccessDeniedException{
		Message_: stringPtr("Access denied"),
	}
	mockService.On("PutLogEvents", mock.MatchedBy(func(req *cloudwatchlogs.PutLogEventsInput) bool {
		return *req.LogGroupName == "group1"
	})).Return((*cloudwatchlogs.PutLogEventsOutput)(nil), accessDeniedErr).Once()

	mockService.On("PutLogEvents", mock.MatchedBy(func(req *cloudwatchlogs.PutLogEventsInput) bool {
		return *req.LogGroupName == "group2"
	})).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Once()

	// Push both batches
	err := heap.Push(batch1)
	assert.NoError(t, err)
	err = heap.Push(batch2)
	assert.NoError(t, err)

	// Process both batches
	processor.processReadyMessages()

	// Wait for async processing to complete
	time.Sleep(100 * time.Millisecond)

	// Verify target1 circuit breaker halted, target2 succeeded
	mu.Lock()
	assert.True(t, halt1Called, "Target1 circuit breaker should halt")
	assert.False(t, resume1Called, "Target1 circuit breaker should not resume")
	assert.True(t, resume2Called, "Target2 should succeed and resume")
	mu.Unlock()

	// Target1 should be back in heap, target2 should be done
	assert.Equal(t, 1, heap.Size(), "Only failed target should remain in heap")

	mockService.AssertExpectations(t)
}

func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
