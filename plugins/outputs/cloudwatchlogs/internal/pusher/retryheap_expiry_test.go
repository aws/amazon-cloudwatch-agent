// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

// TestRetryHeapProcessorExpiredBatchShouldResume demonstrates the bug where
// expired batches don't resume the circuit breaker, leaving the target permanently blocked.
//
// From PR comment: "Say a bad batch from a target caused this to halt. Now that bad batch
// is re-tried for 14 days and eventually dropped - but this never gets resumed in that case right?
// So this target is blocked forever in that scenario?"
func TestRetryHeapProcessorExpiredBatchShouldResume(t *testing.T) {
	logger := testutil.NewNopLogger()

	var sendAttempts atomic.Int32
	mockService := &stubLogsService{
		ple: func(input *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
			sendAttempts.Add(1)
			// Always fail to simulate a problematic target
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

	target := Target{Group: "failing-group", Stream: "stream"}

	// Create retry heap and processor with very short expiry for testing
	retryHeap := NewRetryHeap(10, logger)
	workerPool := NewWorkerPool(5)
	tm := NewTargetManager(logger, mockService)
	maxRetryDuration := 50 * time.Millisecond // Normally 14 days

	retryHeapProcessor := NewRetryHeapProcessor(retryHeap, workerPool, mockService, tm, logger, maxRetryDuration, nil)
	retryHeapProcessor.Start()

	defer retryHeap.Stop()
	defer workerPool.Stop()
	defer retryHeapProcessor.Stop()

	// Create a batch that will expire
	batch := newLogEventBatch(target, nil)
	batch.append(newLogEvent(time.Now(), "test message", nil))

	// Set up callbacks to track circuit breaker state
	var circuitBreakerHalted atomic.Bool
	var circuitBreakerResumed atomic.Bool

	batch.addFailCallback(func() {
		circuitBreakerHalted.Store(true)
	})

	batch.addDoneCallback(func() {
		circuitBreakerResumed.Store(true)
	})

	// Initialize the batch's start time to make it already expired
	batch.initializeStartTime()
	batch.expireAfter = time.Now().Add(-10 * time.Millisecond) // Already expired

	// Update retry metadata to simulate a failed attempt and make it ready for retry
	batch.updateRetryMetadata(&cloudwatchlogs.ServiceUnavailableException{})
	// Set nextRetryTime to past so it's ready for retry
	batch.nextRetryTime = time.Now().Add(-10 * time.Millisecond)

	// Push the expired batch to the retry heap
	err := retryHeap.Push(batch)
	assert.NoError(t, err)

	// Verify batch is in the heap
	assert.Equal(t, 1, retryHeap.Size())

	// Wait for RetryHeapProcessor to process the expired batch
	time.Sleep(200 * time.Millisecond)

	// The batch should have been removed from the heap
	assert.Equal(t, 0, retryHeap.Size(), "Expired batch should be removed from heap")

	// The circuit breaker SHOULD be resumed when the batch expires
	// This allows the target to continue processing new batches after the bad batch is dropped
	assert.True(t, circuitBreakerResumed.Load(),
		"Circuit breaker should be resumed after batch expiry. "+
			"When a batch is retried for 14 days and eventually dropped, "+
			"the target must be unblocked to allow new batches to be processed. "+
			"Otherwise the target remains blocked forever.")
}
