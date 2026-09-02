// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

const eventCount = 100000

func TestPusher(t *testing.T) {
	t.Run("WithSender", func(t *testing.T) {
		t.Parallel()
		var wg sync.WaitGroup
		pusher := setupPusher(t, nil, &wg)

		var completed atomic.Int32
		generateEvents(t, pusher, &completed)

		pusher.Stop()
		wg.Wait()
	})

	t.Run("WithSenderPool", func(t *testing.T) {
		t.Parallel()
		var wg sync.WaitGroup
		wp := NewWorkerPool(5)
		pusher := setupPusher(t, wp, &wg)

		_, isSenderPool := pusher.Sender.(*senderPool)
		assert.True(t, isSenderPool)

		var completed atomic.Int32
		generateEvents(t, pusher, &completed)

		pusher.Stop()
		wg.Wait()
		wp.Stop()
	})
}

func TestPusherStop(t *testing.T) {
	var wg sync.WaitGroup

	s := &mockSender{}
	s.On("Stop").Return()

	logger := testutil.NewNopLogger()
	target := Target{}
	service := new(stubLogsService)
	service.ple = func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}
	mockManager := new(mockTargetManager)
	q := newQueue(logger, target, time.Second, nil, s, &wg)
	pusher := &Pusher{
		Target:         target,
		Queue:          q,
		Service:        service,
		TargetManager:  mockManager,
		EntityProvider: nil,
		Sender:         s,
	}

	pusher.Stop()

	s.AssertCalled(t, "Stop")

}

func generateEvents(t *testing.T, pusher *Pusher, completed *atomic.Int32) {
	t.Helper()
	for i := 0; i < eventCount; i++ {
		pusher.AddEvent(&stubLogEvent{
			message:   "test message",
			timestamp: time.Now(),
			done: func() {
				completed.Add(1)
			},
		})
	}
}

func setupPusher(t *testing.T, workerPool WorkerPool, wg *sync.WaitGroup) *Pusher {
	t.Helper()
	logger := testutil.NewNopLogger()
	target := Target{Group: "G", Stream: "S", Retention: 7}
	service := new(stubLogsService)
	service.ple = func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		// add latency
		time.Sleep(50 * time.Millisecond)
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}
	mockManager := new(mockTargetManager)
	mockManager.On("PutRetentionPolicy", target).Return()

	pusher := NewPusher(
		logger,
		target,
		service,
		mockManager,
		nil,
		workerPool,
		time.Second,
		wg,
		nil, // retryHeap
	)

	assert.NotNil(t, pusher)
	assert.Equal(t, target, pusher.Target)
	assert.NotNil(t, pusher.Queue)
	assert.NotNil(t, pusher.Sender)

	// Verify that PutRetentionPolicy was called
	mockManager.AssertCalled(t, "PutRetentionPolicy", target)
	return pusher
}

func TestPusherRetryHeap(t *testing.T) {
	logger := testutil.NewNopLogger()
	target := Target{Group: "G", Stream: "S"}
	service := &stubLogsService{}
	mockManager := new(mockTargetManager)
	mockManager.On("PutRetentionPolicy", target).Return()

	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()

	retryHeap := NewRetryHeap(logger)
	defer retryHeap.Stop()

	var wg sync.WaitGroup
	pusher := NewPusher(
		logger,
		target,
		service,
		mockManager,
		nil,
		workerPool,
		time.Second,
		&wg,
		retryHeap,
	)

	assert.NotNil(t, pusher)
	assert.Equal(t, target, pusher.Target)

	// The point of this pusher variant: the retryHeap passed to NewPusher must be wired into
	// the underlying sender so failed batches go to the shared heap instead of busy-waiting.
	sp, ok := pusher.Sender.(*senderPool)
	require.True(t, ok, "with a worker pool the pusher's Sender must be a *senderPool")
	inner, ok := sp.sender.(*sender)
	require.True(t, ok, "senderPool must wrap the concrete *sender")
	require.NotNil(t, inner.retryHeap, "NewPusher must wire a retry heap into the underlying sender")
	assert.Same(t, retryHeap, inner.retryHeap,
		"NewPusher must wire the passed retry heap into the underlying sender")

	mockManager.AssertCalled(t, "PutRetentionPolicy", target)
}
