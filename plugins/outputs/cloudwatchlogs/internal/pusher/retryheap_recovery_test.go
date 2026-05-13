// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
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
