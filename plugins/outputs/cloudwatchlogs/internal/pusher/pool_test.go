// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

func TestWorkerPool(t *testing.T) {
	t.Run("BasicSubmit", func(t *testing.T) {
		pool := NewWorkerPool(3).(*workerPool)
		assert.EqualValues(t, 3, pool.WorkerCount())
		var wg sync.WaitGroup
		var completed atomic.Int32

		for i := 0; i < 10; i++ {
			wg.Add(1)
			pool.Submit(func() {
				defer wg.Done()
				completed.Add(1)
			})
		}

		wg.Wait()
		assert.EqualValues(t, 10, completed.Load())
		assert.EqualValues(t, 3, pool.WorkerCount())
		pool.Stop()
		assert.EqualValues(t, 0, pool.WorkerCount())
	})

	t.Run("GracefulStop", func(t *testing.T) {
		pool := NewWorkerPool(20)

		var completed atomic.Int32
		taskCount := 500

		for i := 0; i < taskCount; i++ {
			pool.Submit(func() {
				time.Sleep(time.Millisecond)
				completed.Add(1)
			})
		}

		pool.Stop()
		assert.EqualValues(t, taskCount, completed.Load())
	})

	t.Run("SubmitAfterStop", func(t *testing.T) {
		pool := NewWorkerPool(3).(*workerPool)
		pool.Stop()
		assert.EqualValues(t, 0, pool.WorkerCount())
		assert.NotPanics(t, func() {
			pool.Submit(func() {
				assert.Fail(t, "should not reach")
			})
		})
		time.Sleep(time.Millisecond)
	})

	t.Run("MultipleStops", func(t *testing.T) {
		pool := NewWorkerPool(3)
		assert.NotPanics(t, func() {
			for i := 0; i < 10; i++ {
				pool.Stop()
			}
		})
	})

	t.Run("ConcurrentSubmitAndStop", func(t *testing.T) {
		pool := NewWorkerPool(20)
		var wg sync.WaitGroup
		taskCount := 1000
		var completed atomic.Int32

		// Start submitting tasks
		for i := 0; i < taskCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pool.Submit(func() {
					time.Sleep(time.Millisecond)
					completed.Add(1)
				})
			}()
		}

		// Stop the pool while tasks are being submitted
		time.Sleep(5 * time.Millisecond)
		pool.Stop()

		assert.LessOrEqual(t, completed.Load(), int32(taskCount))
		assert.Greater(t, completed.Load(), int32(0))
	})
}

func TestSenderPool(t *testing.T) {
	logger := testutil.NewNopLogger()
	mockService := new(mockLogsService)
	mockService.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil)
	s := newSender(logger, mockService, nil, nil)
	p := NewWorkerPool(12)
	sp := newSenderPool(p, s)

	// Retry duration methods removed - just test basic functionality

	var completed atomic.Int32
	var evts []*logEvent
	for i := 0; i < 200; i++ {
		evts = append(evts, newLogEvent(time.Now(), "test", func() {
			time.Sleep(time.Millisecond)
			completed.Add(1)
		}))
	}

	for _, evt := range evts {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(evt)
		sp.Send(batch)
	}

	p.Stop()
	s.Stop()
	assert.Equal(t, int32(200), completed.Load())
}

func TestSenderPoolRetryHeap(_ *testing.T) {
	logger := testutil.NewNopLogger()
	mockService := new(mockLogsService)
	mockService.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil)

	// Create RetryHeap
	retryHeap := NewRetryHeap(10)
	defer retryHeap.Stop()

	s := newSender(logger, mockService, nil, retryHeap)
	p := NewWorkerPool(12)
	defer p.Stop()

	sp := newSenderPool(p, s)

	sp.Stop()
}
