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
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
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

// Iteration-3 major: AddEvent was made stop-aware but AddEventNonBlocking (the EMF path)
// was not. startNonBlockCh is UNBUFFERED and its only receiver is the merge loop, which
// exits on stopCh -- so the first EMF event published after shutdown blocks forever while
// cwDest.Publish holds the destination lock, reinstating the deadlock on the EMF path.
func TestAddEventNonBlockingIsStopAware(t *testing.T) {
	logger := testutil.NewNopLogger()
	var wg sync.WaitGroup
	q := newQueue(logger, Target{"G", "S", util.StandardLogGroupClass, -1},
		time.Hour, nil, &stubSender{}, &wg)

	q.Stop() // merge loop exits; nothing will ever receive on startNonBlockCh

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		q.AddEventNonBlocking(newStubLogEvent("MSG", time.Now()))
	}()

	select {
	case <-returned:
		// Returned: the EMF publish path cannot wedge shutdown.
	case <-time.After(15 * time.Second):
		t.Fatal("AddEventNonBlocking blocked after Stop: the unbuffered startNonBlockCh send " +
			"has no receiver once the merge loop exits, so Publish wedges holding cd.Lock")
	}
}

// Iteration-3 major: Submit holds stopLock.RLock while blocking on the task channel, and
// Stop needs stopLock.Lock to close stopCh. The write lock waits for that reader, so Stop
// can never release the blocked Submit -- the pool deadlocks on a saturated shutdown.
func TestWorkerPoolStopUnblocksSaturatedSubmit(t *testing.T) {
	pool := NewWorkerPool(1) // tasks buffer = 2
	release := make(chan struct{})

	// Occupy the worker and fill the buffer.
	pool.Submit(func() { <-release })
	pool.Submit(func() { <-release })
	pool.Submit(func() { <-release })

	blocked := make(chan struct{})
	go func() {
		defer close(blocked)
		pool.Submit(func() {}) // parks: worker busy and buffer full
	}()
	time.Sleep(200 * time.Millisecond) // let it park inside Submit

	// Stop itself still waits on the deliberately-hung worker via wg.Wait(), which is
	// correct. The property under test is that Stop RELEASES the parked Submit: pre-fix
	// Submit held stopLock.RLock, so Stop could not take the write lock to close stopCh
	// and the Submit was stuck forever.
	go pool.Stop()

	select {
	case <-blocked:
		// Submit returned once stopCh closed.
	case <-time.After(15 * time.Second):
		close(release)
		t.Fatal("Stop() did not release the parked Submit: it held stopLock.RLock, so Stop's " +
			"write lock could never be acquired to close stopCh")
	}
	close(release)
}

// Iteration-4 major: Submit's select is random when stopCh is closed AND the task buffer
// still has room, so a task could be enqueued after Stop and never run -- leaving its
// batch unfinalized (breaker halted, offsets never advanced). Submit must answer
// definitively so senderPool can abandon the batch instead.
func TestSubmitRejectsAfterStopAndSenderPoolAbandons(t *testing.T) {
	logger := testutil.NewNopLogger()
	pool := NewWorkerPool(2) // buffer has room, so the race is reachable
	pool.Stop()

	// Submit must refuse deterministically, even with buffer space free.
	for i := 0; i < 50; i++ {
		require.False(t, pool.Submit(func() {}),
			"Submit must refuse tasks once the pool is stopping, even with buffer room")
	}

	// And senderPool must finalize the batch it could not hand off.
	var resumed, stateRuns atomic.Int32
	sp := newSenderPool(pool, &stubSender{}, logger)
	b := readyBatch("g", nil, func() { stateRuns.Add(1) })
	b.addResumeCallback(func() { resumed.Add(1) })
	sp.Send(b)

	require.Equal(t, 1, int(resumed.Load()),
		"a batch that could not be submitted must be abandoned so the breaker clears")
	require.Zero(t, stateRuns.Load(),
		"abandon() must not persist offsets for events that were never sent")
}

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
