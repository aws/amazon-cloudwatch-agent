// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

// M1 (swarm major): senderPool.Send submits sender.Send into the worker pool, and
// workerPool.worker() calls task() with no recover. A panic anywhere in a send -- the API
// call, InitTarget, or a done/state callback -- is therefore an unrecovered panic in a
// goroutine, which terminates the whole agent process. flushBatch's recover only covers
// the pop/expire/submit step, not the send that runs later in the worker.
func TestWorkerPoolSurvivesPanickingTask(t *testing.T) {
	pool := NewWorkerPool(1)
	defer pool.Stop()

	var ran atomic.Int32
	pool.Submit(func() { panic("task boom") })
	// A later task must still execute: the worker has to survive the panic above.
	pool.Submit(func() { ran.Add(1) })

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && ran.Load() == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(t, 1, int(ran.Load()),
		"a panicking task must not kill the worker; without a recover in worker() the "+
			"panic is unrecovered and the agent process exits")
}

// Iteration-2 major: runTask's recover keeps the worker alive but has no access to the
// batch, so a panic mid-send left the batch unfinalized -- breaker halted forever and file
// offsets never advanced. senderPool.Send now recovers where the batch is in scope and
// abandons it (clears the breaker, does NOT persist offsets).
func TestSenderPoolAbandonsBatchOnPanic(t *testing.T) {
	logger := testutil.NewNopLogger()
	pool := NewWorkerPool(1)
	defer pool.Stop()

	var resumed, stateRuns atomic.Int32
	sp := newSenderPool(pool, panicSender{}, logger)

	b := readyBatch("g", nil, func() { stateRuns.Add(1) })
	b.addResumeCallback(func() { resumed.Add(1) })
	sp.Send(b)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && resumed.Load() == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(t, 1, int(resumed.Load()),
		"a panic mid-send must still clear the target's circuit breaker via abandon()")
	require.Zero(t, stateRuns.Load(),
		"abandon() must NOT persist offsets: the events were never delivered")
}

// panicSender panics inside Send to simulate a panic in the API call or a callback.
type panicSender struct{}

func (panicSender) Send(*logEventBatch) { panic("send boom") }
func (panicSender) Stop()               {}

// M3 (swarm major): RetryHeapProcessor.Stop() flushes ready batches through
// senderPool.Send -> workerPool.Submit, which blocks while the pool is saturated. The
// pool is still running at that point (Close stops the processor BEFORE the pool), so
// Submit has no stopCh to fall through and shutdown can hang unbounded.
func TestRetryHeapProcessorStopDoesNotBlockOnSaturatedPool(t *testing.T) {
	logger := testutil.NewNopLogger()
	svc := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})

	pool := NewWorkerPool(1) // tasks buffer = size*2 = 2
	defer pool.Stop()

	// Occupy the single worker and fill the task buffer so any further Submit blocks.
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	pool.Submit(func() { defer wg.Done(); <-release })
	pool.Submit(func() { <-release })
	pool.Submit(func() { <-release })

	retryHeap := NewRetryHeap(logger)
	defer retryHeap.Stop()
	p := NewRetryHeapProcessor(retryHeap, pool, svc, NewTargetManager(logger, svc), logger, nil)

	// Ready batches force Stop()'s flush to Submit into the saturated pool.
	for i := 0; i < 3; i++ {
		require.NoError(t, retryHeap.Push(readyBatch("g", nil, nil)))
	}

	stopped := make(chan struct{})
	go func() {
		p.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		// Stop returned: shutdown is bounded even with a saturated pool.
	case <-time.After(20 * time.Second):
		close(release)
		wg.Wait()
		t.Fatal("RetryHeapProcessor.Stop() blocked >20s flushing into a saturated worker " +
			"pool; shutdown must be bounded")
	}

	close(release)
	wg.Wait()
}
