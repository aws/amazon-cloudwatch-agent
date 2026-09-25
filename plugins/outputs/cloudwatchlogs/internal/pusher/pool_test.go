// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

func TestWorkerPool(t *testing.T) {
	t.Run("BasicSubmit", func(t *testing.T) {
		pool := NewWorkerPool(3, testutil.NewNopLogger()).(*workerPool)
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
		pool := NewWorkerPool(20, testutil.NewNopLogger())

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
		pool := NewWorkerPool(3, testutil.NewNopLogger()).(*workerPool)
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
		pool := NewWorkerPool(3, testutil.NewNopLogger())
		assert.NotPanics(t, func() {
			for i := 0; i < 10; i++ {
				pool.Stop()
			}
		})
	})

	t.Run("ConcurrentSubmitAndStop", func(t *testing.T) {
		pool := NewWorkerPool(20, testutil.NewNopLogger())
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
	mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil)
	s := newSender(logger, mockService, nil, nil)
	p := NewWorkerPool(12, testutil.NewNopLogger())
	sp := newSenderPool(p, s, testutil.NewNopLogger())

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

// The worker pool must behave across the concurrency values operators actually set,
// including more workers than CPUs.
func TestWorkerPoolConcurrencyMatrix(t *testing.T) {
	for _, size := range []int{1, 2, 8, runtime.NumCPU() * 4} {
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			var ran atomic.Int32
			pool := NewWorkerPool(size, testutil.NewNopLogger())
			total := size * 5
			var wg sync.WaitGroup
			for i := 0; i < total; i++ {
				wg.Add(1)
				pool.Submit(func() { defer wg.Done(); ran.Add(1) })
			}
			finished := make(chan struct{})
			go func() { wg.Wait(); close(finished) }()
			select {
			case <-finished:
			case <-time.After(20 * time.Second):
				pool.Stop()
				t.Fatalf("worker pool stalled at size %d (%d/%d tasks ran)", size, ran.Load(), total)
			}
			pool.Stop()
			assert.Equal(t, total, int(ran.Load()), "all tasks should run at pool size %d", size)
		})
	}
}

// senderPool.Send submits sender.Send into the worker pool, and
// workerPool.worker() calls task() with no recover. A panic anywhere in a send -- the API
// call, InitTarget, or a done/state callback -- is therefore an unrecovered panic in a
// goroutine, which terminates the whole agent process. flushBatch's recover only covers
// the pop/expire/submit step, not the send that runs later in the worker.
func TestWorkerPoolSurvivesPanickingTask(t *testing.T) {
	pool := NewWorkerPool(1, testutil.NewNopLogger())
	defer pool.Stop()

	var ran atomic.Int32
	pool.Submit(func() { panic("task boom") })
	// A later task must still execute: the worker has to survive the panic above.
	pool.Submit(func() { ran.Add(1) })

	require.Eventually(t, func() bool { return ran.Load() == 1 }, 10*time.Second, 20*time.Millisecond,
		"a panicking task must not kill the worker; without a recover in worker() the "+
			"panic is unrecovered and the agent process exits")
}

// runTask's recover keeps the worker alive but has no access to the
// batch, so a panic mid-send left the batch unfinalized -- breaker halted forever and file
// offsets never advanced. senderPool.Send now recovers where the batch is in scope and
// abandons it (clears the breaker, does NOT persist offsets).
func TestSenderPoolAbandonsBatchOnPanic(t *testing.T) {
	logger := testutil.NewNopLogger()
	pool := NewWorkerPool(1, testutil.NewNopLogger())
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

// Stop must release a Submit parked on a saturated pool: Submit selects on stopCh, so
// closing stopCh wakes it. A lock-guarded Submit would instead deadlock against Stop.
func TestWorkerPoolStopUnblocksSaturatedSubmit(t *testing.T) {
	pool := NewWorkerPool(1, testutil.NewNopLogger()) // tasks buffer = 2
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

	// Stop still waits on the hung worker via wg.Wait() (correct); the property under test
	// is that Stop RELEASES the parked Submit by closing stopCh.
	go pool.Stop()

	select {
	case <-blocked:
		// Submit returned once stopCh closed.
	case <-time.After(15 * time.Second):
		close(release)
		t.Fatal("Stop() did not release the parked Submit by closing stopCh")
	}
	close(release)
}

// Submit's select is random when stopCh is closed AND the task buffer
// still has room, so a task could be enqueued after Stop and never run -- leaving its
// batch unfinalized (breaker halted, offsets never advanced). Submit must answer
// definitively so senderPool can abandon the batch instead.
func TestSubmitRejectsAfterStopAndSenderPoolAbandons(t *testing.T) {
	logger := testutil.NewNopLogger()
	pool := NewWorkerPool(2, testutil.NewNopLogger()) // buffer has room, so the race is reachable
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

// Regression test for the shutdown TOCTOU in Submit. The stopping guard can observe
// stopping==false, then Stop can run to completion -- every worker drains the buffer, hits
// the default branch, and exits -- before Submit reaches its select. Because select is
// random when both cases are ready, the buffered send can still win over the closed stopCh,
// so Submit returns true for a task that no worker will ever run, and senderPool.Send then
// leaves the batch unfinalized. The invariant under test: every task Submit reports accepted
// is eventually run. This fails against the guard-only code (accepted tasks are orphaned in
// the buffer) and passes once Submit re-checks stopping after the send wins.
func TestSubmitAcceptedTaskAlwaysRuns(t *testing.T) {
	const iterations = 300
	for i := 0; i < iterations; i++ {
		pool := NewWorkerPool(2, testutil.NewNopLogger()) // buffer room keeps the race reachable
		var accepted, ran atomic.Int64

		var submitters sync.WaitGroup
		for s := 0; s < 4; s++ {
			submitters.Add(1)
			go func() {
				defer submitters.Done()
				for {
					if pool.Submit(func() { ran.Add(1) }) {
						accepted.Add(1)
						continue
					}
					return // pool refused: it is shutting down
				}
			}()
		}

		// Let the submitters saturate the pool, then stop concurrently so the guard/Stop
		// window is exercised while sends are in flight.
		runtime.Gosched()
		pool.Stop()       // returns only after workers drain the buffer and exit
		submitters.Wait() // every Submit has returned, so accepted is final

		// Stop has joined all workers and Submit now refuses, so every accepted task has had
		// its chance to run. A task reported accepted but missing from the run count was
		// orphaned in the buffer with no worker to execute it.
		require.GreaterOrEqualf(t, ran.Load(), accepted.Load(),
			"iteration %d: %d tasks were accepted but only %d ran; Submit reported a task "+
				"accepted that no worker executed", i, accepted.Load(), ran.Load())
	}
}

// doneSender simulates a worker that successfully delivered the batch: it finalizes via
// done(). Used to race a draining worker's done() against senderPool's abandon().
type doneSender struct{}

func (doneSender) Send(b *logEventBatch) { b.done() }

func (doneSender) Stop() {}

// The Submit stop-race fix trades an orphaned task for a possible double-finalize: Submit's
// send can win the buffer, then the re-check observes stopping and returns false, so
// senderPool.Send abandons the batch -- yet a draining worker can still pull that already
// buffered task and run it to done(). Both a terminal from the worker (done) and a terminal
// from the caller (abandon) then fire on the same batch. The idempotency guard makes
// terminal finalization first-wins/second-no-op, so every submitted batch is finalized
// EXACTLY once: never twice (double-finalize) and never zero times (orphaned/unfinalized).
// This fails if the CAS guards in done/drop/abandon are removed (batches finalize twice).
func TestConcurrentSubmitStopFinalizesEachBatchExactlyOnce(t *testing.T) {
	const (
		iterations     = 200
		batchesPerIter = 64
		submitters     = 4
	)
	for i := 0; i < iterations; i++ {
		pool := NewWorkerPool(2, testutil.NewNopLogger()) // buffer room keeps the race reachable
		sp := newSenderPool(pool, doneSender{}, testutil.NewNopLogger())

		// Per-batch finalize counter: done() runs the done callback, abandon() runs the
		// resume callback. Whichever terminal wins the CAS increments exactly once.
		counters := make([]atomic.Int32, batchesPerIter)
		batches := make([]*logEventBatch, batchesPerIter)
		for b := range batches {
			c := &counters[b]
			batch := readyBatch("g", func() { c.Add(1) }, nil)
			batch.addResumeCallback(func() { c.Add(1) })
			batches[b] = batch
		}

		var wg sync.WaitGroup
		for s := 0; s < submitters; s++ {
			wg.Add(1)
			go func(start int) {
				defer wg.Done()
				for b := start; b < batchesPerIter; b += submitters {
					sp.Send(batches[b]) // finalizes via done() (accepted+run) or abandon() (rejected)
				}
			}(s)
		}

		// Stop concurrently so the buffer/drain/re-check window overlaps in-flight sends.
		runtime.Gosched()
		pool.Stop() // returns only after workers drain the buffer and exit
		wg.Wait()   // every sp.Send has returned, so every abandon() has run

		// Stop joined all workers (all drained done() calls ran) and every submitter finished
		// (all abandon() calls ran), so each batch's finalize count is final.
		for b := range batches {
			require.Equalf(t, int32(1), counters[b].Load(),
				"iteration %d batch %d finalized %d times; terminal finalization must be "+
					"exactly-once (1=ok, 2=double-finalize, 0=orphaned)", i, b, counters[b].Load())
		}
	}
}
