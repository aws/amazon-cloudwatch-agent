// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

const eventCount = 100000

func TestPusher(t *testing.T) {
	t.Run("WithSender", func(t *testing.T) {
		t.Parallel()
		stop := make(chan struct{})
		var wg sync.WaitGroup
		pusher := setupPusher(t, nil, stop, &wg)

		var completed atomic.Int32
		generateEvents(t, pusher, &completed)

		close(stop)
		wg.Wait()
	})

	t.Run("WithSenderPool", func(t *testing.T) {
		t.Parallel()
		stop := make(chan struct{})
		var wg sync.WaitGroup
		wp := NewWorkerPool(5)
		pusher := setupPusher(t, wp, stop, &wg)

		_, isSenderPool := pusher.Sender.(*senderPool)
		assert.True(t, isSenderPool)

		var completed atomic.Int32
		generateEvents(t, pusher, &completed)

		close(stop)
		wg.Wait()
		wp.Stop()
	})
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

func setupPusher(t *testing.T, workerPool WorkerPool, stop chan struct{}, wg *sync.WaitGroup) *Pusher {
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
		time.Minute,
		stop,
		wg,
	)

	assert.NotNil(t, pusher)
	assert.Equal(t, target, pusher.Target)
	assert.NotNil(t, pusher.Queue)
	assert.NotNil(t, pusher.Sender)

	// Verify that PutRetentionPolicy was called
	mockManager.AssertCalled(t, "PutRetentionPolicy", target)
	return pusher
}
