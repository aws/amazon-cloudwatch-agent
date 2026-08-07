// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

// P9: main's caller-supplied retryDuration was replaced by a fixed ceiling when the RetryHeap
// landed (pool.go lost SetRetryDuration/RetryDuration). Verify the ceiling is actually applied,
// since nothing else now bounds how long a batch is retried.
func TestRetryCeilingIsApplied(t *testing.T) {
	b := newLogEventBatch(Target{Group: "g", Stream: "s"}, nil)
	b.append(newLogEvent(time.Now(), "payload", func() {}))

	require.True(t, b.expireAfter.IsZero(), "expiry should not be set before the first send")
	b.initializeStartTime()
	require.False(t, b.expireAfter.IsZero(), "expiry must be stamped on the first send")

	assert.Equal(t, maxRetryTimeout, b.expireAfter.Sub(b.startTime),
		"retry ceiling must equal maxRetryTimeout now that retryDuration is gone")
	assert.False(t, b.isExpired(), "a fresh batch must not be expired")

	b.expireAfter = time.Now().Add(-time.Second)
	assert.True(t, b.isExpired(), "a batch past its ceiling must expire")
}

// N1: Stop() must be idempotent and safe under concurrent callers. The fix replaced a mutex
// with sync.Once; a regression here would double-close stopCh and panic.
func TestProcessorStopIsIdempotentAndConcurrencySafe(t *testing.T) {
	logger := testutil.NewNopLogger()
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})
	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()
	retryHeap := NewRetryHeap(logger)

	p := NewRetryHeapProcessor(retryHeap, workerPool, service, NewTargetManager(logger, service), logger, nil)
	p.Start()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); p.Stop() }()
	}
	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()

	select {
	case <-finished:
	case <-time.After(20 * time.Second):
		t.Fatal("concurrent Stop() calls deadlocked")
	}
	assert.True(t, p.stopped.Load(), "processor should be marked stopped")
	p.Stop() // once more, after the fact
}

// N2: pushing to a closed heap must fail cleanly so the caller can abandon rather than block
// or panic. This is the path sender.Send() takes during shutdown.
func TestHeapPushAfterStopFailsCleanly(t *testing.T) {
	logger := testutil.NewNopLogger()
	h := NewRetryHeap(logger)
	require.NoError(t, h.Push(readyBatch("g", nil, nil)))
	h.Stop()

	err := h.Push(readyBatch("g", nil, nil))
	require.Error(t, err, "push after Stop must report failure, not silently succeed")
	assert.Contains(t, err.Error(), "stopped")
	h.Stop() // idempotent
}

// N3: a halted target must not wedge shutdown. waitIfHalted selects on stopCh as well as the
// resume channel, so Stop() has to release a queue that is parked on the circuit breaker.
func TestHaltedQueueStillShutsDown(t *testing.T) {
	logger := testutil.NewNopLogger()
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	})
	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()
	retryHeap := NewRetryHeap(logger)
	tm := NewTargetManager(logger, service)

	var wg sync.WaitGroup
	pusher := NewPusher(logger, Target{Group: "g", Stream: "s"}, service, tm, nil,
		workerPool, 20*time.Millisecond, &wg, retryHeap)

	for i := 0; i < 5; i++ {
		pusher.AddEvent(newStubLogEvent("halt-me", time.Now()))
	}
	time.Sleep(500 * time.Millisecond) // let a failure latch the breaker

	stopped := make(chan struct{})
	go func() { pusher.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(20 * time.Second):
		t.Fatal("Stop() hung on a halted queue: waitIfHalted did not observe stopCh")
	}
	retryHeap.Stop()
}

// N4: the worker pool must behave across the concurrency values operators actually set,
// including more workers than CPUs.
func TestWorkerPoolConcurrencyMatrix(t *testing.T) {
	for _, size := range []int{1, 2, 8, runtime.NumCPU() * 4} {
		var ran atomic.Int32
		pool := NewWorkerPool(size)
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
	}
}

// N5: a permanently invalid batch must advance offsets so it is not re-read forever (its own
// poison pill), while still clearing the breaker.
func TestInvalidBatchAdvancesOffsetsAndResumes(t *testing.T) {
	logger := testutil.NewNopLogger()
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return nil, &cloudwatchlogs.InvalidParameterException{}
	})
	retryHeap := NewRetryHeap(logger)
	defer retryHeap.Stop()

	var delivered, offsets, resumed atomic.Int32
	batch := readyBatch("g", func() { delivered.Add(1) }, func() { offsets.Add(1) })
	batch.addResumeCallback(func() { resumed.Add(1) })

	newSender(logger, service, NewTargetManager(logger, service), retryHeap).Send(batch)

	assert.Zero(t, delivered.Load(), "an invalid batch was never delivered")
	assert.Equal(t, int32(1), offsets.Load(), "offsets must advance so the batch is not re-read forever")
	assert.Equal(t, int32(1), resumed.Load(), "the circuit breaker must be cleared")
}

// N6: every target failing at once must not deadlock or grow without bound.
func TestAllTargetsFailingDoesNotDeadlock(t *testing.T) {
	logger := testutil.NewNopLogger()
	var calls atomic.Int32
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		calls.Add(1)
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	})
	workerPool := NewWorkerPool(4)
	retryHeap := NewRetryHeap(logger)
	tm := NewTargetManager(logger, service)
	p := NewRetryHeapProcessor(retryHeap, workerPool, service, tm, logger, nil)
	p.Start()

	var wg sync.WaitGroup
	pushers := make([]*Pusher, 0, 6)
	for i := 0; i < 6; i++ {
		grp := "failing-" + string(rune('a'+i))
		pushers = append(pushers, NewPusher(logger, Target{Group: grp, Stream: "s"}, service, tm, nil,
			workerPool, 20*time.Millisecond, &wg, retryHeap))
	}
	for _, ps := range pushers {
		for i := 0; i < 10; i++ {
			ps.AddEvent(newStubLogEvent("fail", time.Now()))
		}
	}
	require.Eventually(t, func() bool { return calls.Load() > 0 }, 10*time.Second, 50*time.Millisecond)
	time.Sleep(2 * time.Second)

	// breaker caps each failing target at roughly one in-flight batch
	assert.LessOrEqual(t, retryHeap.Size(), 24, "heap grew beyond one batch per target: %d", retryHeap.Size())

	done := make(chan struct{})
	go func() {
		for _, ps := range pushers {
			ps.Stop()
		}
		p.Stop()
		workerPool.Stop()
		retryHeap.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(40 * time.Second):
		t.Fatal("shutdown deadlocked with all targets failing")
	}
}

// N7: destinations churning while the service throttles exercises the #2190 seam -- the
// TargetManager's shared retryer must outlive individual destination stops.
func TestChurnWhileThrottledDoesNotWedge(t *testing.T) {
	logger := testutil.NewNopLogger()
	var calls atomic.Int32
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if calls.Add(1)%3 == 0 {
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		}
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	})
	workerPool := NewWorkerPool(4)
	retryHeap := NewRetryHeap(logger)
	tm := NewTargetManager(logger, service)
	p := NewRetryHeapProcessor(retryHeap, workerPool, service, tm, logger, nil)
	p.Start()

	var wg sync.WaitGroup
	for round := 0; round < 8; round++ {
		grp := "churn-" + string(rune('a'+round))
		ps := NewPusher(logger, Target{Group: grp, Stream: "s"}, service, tm, nil,
			workerPool, 20*time.Millisecond, &wg, retryHeap)
		ps.AddEvent(newStubLogEvent("churn", time.Now()))
		time.Sleep(120 * time.Millisecond)
		ps.Stop() // destination goes away mid-flight
	}

	// the shared TargetManager must still work after all those stops
	require.NoError(t, tm.InitTarget(Target{Group: "after-churn", Stream: "s"}),
		"TargetManager broke after destination churn: the #2190 shared retryer was tied to a stopped dest")

	p.Stop()
	workerPool.Stop()
	retryHeap.Stop()
}
