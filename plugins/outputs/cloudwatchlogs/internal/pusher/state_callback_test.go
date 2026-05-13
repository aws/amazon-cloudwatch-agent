// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

type mockFileRangeQueue struct {
	mock.Mock
}

func (m *mockFileRangeQueue) ID() string {
	return m.Called().String(0)
}

func (m *mockFileRangeQueue) Enqueue(r state.Range) {
	m.Called(r)
}

// newStatefulBatch creates a batch with stateful events that register state callbacks.
func newStatefulBatch(target Target, queue *mockFileRangeQueue) *logEventBatch {
	batch := newLogEventBatch(target, nil)
	now := time.Now()
	evt := newStatefulLogEvent(now, "test", nil, &logEventState{
		r:     state.NewRange(0, 100),
		queue: queue,
	})
	batch.append(evt)
	return batch
}

// TestRetryHeapSuccessCallsStateCallback verifies that when a batch succeeds
// on retry through the heap, state callbacks fire to persist file offsets.
func TestRetryHeapSuccessCallsStateCallback(t *testing.T) {
	logger := testutil.NewNopLogger()
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
	workerPool := NewWorkerPool(2)
	tm := NewTargetManager(logger, service)
	defer retryHeap.Stop()
	defer workerPool.Stop()

	processor := NewRetryHeapProcessor(retryHeap, workerPool, service, tm, logger, retryer.NewLogThrottleRetryer(logger))

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
	logger := testutil.NewNopLogger()
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
	workerPool := NewWorkerPool(2)
	tm := NewTargetManager(logger, service)
	defer retryHeap.Stop()
	defer workerPool.Stop()

	processor := NewRetryHeapProcessor(retryHeap, workerPool, service, tm, logger, nil)

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
	logger := testutil.NewNopLogger()
	target := Target{Group: "group", Stream: "stream"}

	var stateCallCount atomic.Int32

	retryHeap := NewRetryHeap(logger)
	workerPool := NewWorkerPool(2)
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

	processor := NewRetryHeapProcessor(retryHeap, workerPool, service, tm, logger, nil)
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
	retryHeap.Stop()

	assert.Equal(t, int32(0), stateCallCount.Load(),
		"State callback should not be called for unprocessed batches during shutdown")
	assert.Equal(t, 1, retryHeap.Size(), "Batch should remain in heap after shutdown")
}
