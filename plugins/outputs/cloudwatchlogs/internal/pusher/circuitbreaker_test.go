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
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

// TestCircuitBreakerBlocksTargetAfterFailure verifies that when a batch fails
// for a target, the circuit breaker prevents additional batches from that target
// from being sent until the failing batch is retried successfully.
//
// Without a circuit breaker, a problematic target continues producing new batches
// that flood the SenderQueue/WorkerPool, starving healthy targets.
func TestCircuitBreakerBlocksTargetAfterFailure(t *testing.T) {
	logger := testutil.NewNopLogger()

	failingTarget := Target{Group: "failing-group", Stream: "stream"}
	healthyTarget := Target{Group: "healthy-group", Stream: "stream"}

	var failingTargetSendCount atomic.Int32
	var healthyTargetSendCount atomic.Int32

	service := &stubLogsService{
		ple: func(input *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
			if *input.LogGroupName == failingTarget.Group {
				failingTargetSendCount.Add(1)
				return nil, &cloudwatchlogs.ServiceUnavailableException{}
			}
			healthyTargetSendCount.Add(1)
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

	concurrency := 5
	workerPool := NewWorkerPool(concurrency)
	retryHeap := NewRetryHeap(logger)
	defer workerPool.Stop()
	defer retryHeap.Stop()

	tm := NewTargetManager(logger, service)

	var wg sync.WaitGroup
	flushTimeout := 50 * time.Millisecond

	failingPusher := NewPusher(logger, failingTarget, service, tm, nil, workerPool, flushTimeout, &wg, retryHeap)
	healthyPusher := NewPusher(logger, healthyTarget, service, tm, nil, workerPool, flushTimeout, &wg, retryHeap)
	defer failingPusher.Stop()
	defer healthyPusher.Stop()

	now := time.Now()

	// Send events to both targets. The failing target will fail on PutLogEvents,
	// and the circuit breaker should block it from sending more batches.
	for i := 0; i < 10; i++ {
		failingPusher.AddEvent(newStubLogEvent("fail", now))
		healthyPusher.AddEvent(newStubLogEvent("ok", now))
	}

	// Wait for flushes to occur
	time.Sleep(500 * time.Millisecond)

	// Send more events - the failing target should be blocked by circuit breaker
	for i := 0; i < 10; i++ {
		failingPusher.AddEvent(newStubLogEvent("fail-more", now))
		healthyPusher.AddEvent(newStubLogEvent("ok-more", now))
	}

	time.Sleep(500 * time.Millisecond)

	// Circuit breaker assertion: after the first failure, the failing target should
	// NOT have sent additional batches. Only 1 send attempt should have been made
	// before the circuit breaker blocks it.
	assert.Equal(t, int32(1), failingTargetSendCount.Load(),
		"Circuit breaker should block failing target after exactly 1 send attempt")

	// Healthy target should continue sending successfully
	assert.Greater(t, healthyTargetSendCount.Load(), int32(0),
		"Healthy target should continue sending while failing target is blocked")
}

// The circuit breaker halts a target on failure and only clears on
// batch.done(). Terminal paths in sender.Send() (non-AWS error, InvalidParameter /
// DataAlreadyAccepted, expiry) call updateState() and return without resuming, so the
// target's queue stays halted and stops shipping until the agent restarts.
func TestTerminalFailureResumesHaltedTarget(t *testing.T) {
	logger := testutil.NewNopLogger()

	var calls, delivered atomic.Int32
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		switch calls.Add(1) {
		case 1:
			return nil, &cloudwatchlogs.ServiceUnavailableException{} // retryable -> heap -> fail() -> halt
		case 2:
			return nil, &cloudwatchlogs.InvalidParameterException{} // terminal -> updateState, no resume
		default:
			delivered.Add(1)
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		}
	})

	workerPool := NewWorkerPool(2)
	defer workerPool.Stop()
	retryHeap := NewRetryHeap(logger)
	tm := NewTargetManager(logger, service)

	p := NewRetryHeapProcessor(retryHeap, workerPool, service, tm, logger, nil)
	p.Start()

	var wg sync.WaitGroup
	pusher := NewPusher(logger, Target{Group: "g", Stream: "stream"}, service, tm, nil,
		workerPool, 20*time.Millisecond, &wg, retryHeap)

	// First batch fails retryably (halts the target), then its retry hits the terminal error.
	pusher.AddEvent(newStubLogEvent("first", time.Now()))
	require.Eventually(t, func() bool { return calls.Load() >= 2 }, 10*time.Second, 20*time.Millisecond,
		"test setup: expected a retryable failure followed by a terminal failure")

	// The target must be shipping again. Pre-fix the terminal path never resumes, so the
	// queue stays halted, send() blocks in waitIfHalted(), and nothing is ever delivered.
	for i := 0; i < 5; i++ {
		pusher.AddEvent(newStubLogEvent("after-terminal", time.Now()))
	}
	assert.Eventually(t, func() bool { return delivered.Load() > 0 }, 10*time.Second, 50*time.Millisecond,
		"target never resumed after a terminal (non-retryable) failure: the circuit breaker "+
			"latched permanently -- waitIfHalted() blocks send(), eventsCh fills, and AddEvent "+
			"blocks the tailer for this log group until restart")

	pusher.Stop()
	p.Stop()
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

// KNOWN LIMITATION (documented, not fixed): the circuit breaker is a
// single bool per target, so ANY batch's resume() unhalts the queue even while other
// batches for the same target are still failed and queued in the retry heap. The breaker
// therefore bounds in-flight failed batches per target only approximately, rather than
// guaranteeing one.
//
// Not converted to a refcount because halt/resume are not 1:1: fail() invokes q.halt() on
// EVERY failure (a batch retried 5x halts 5x) while resume() runs once per finalization,
// and onSuccessCallback calls q.resume() directly outside batch.resume(). A counter would
// need fail() made idempotent per batch plus the success path rerouted; if the count ever
// failed to reach zero the target would halt forever -- a worse failure than coarseness.
//
// This test pins the CURRENT behavior so a future refcount change is a deliberate,
// visible edit rather than a silent semantic drift.
func TestBreakerIsClearedByAnyBatchResume_KnownLimitation(t *testing.T) {
	logger := testutil.NewNopLogger()
	var wg sync.WaitGroup
	mockSender := &stubSender{}
	q := newQueue(logger, Target{"G", "S", util.StandardLogGroupClass, -1},
		10*time.Millisecond, nil, mockSender, &wg).(*queue)

	// Batch #1 for this target fails and halts the breaker.
	q.halt()
	q.haltMu.Lock()
	haltedAfterFail := q.halted
	q.haltMu.Unlock()
	require.True(t, haltedAfterFail, "sanity: fail() must halt the target")

	// Batch #2 for the SAME target is abandoned (transient give-up) and calls resume().
	b2 := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
	b2.append(newLogEvent(time.Now(), "payload", func() {}))
	b2.addResumeCallback(q.resume)
	b2.abandon()

	q.haltMu.Lock()
	stillHalted := q.halted
	q.haltMu.Unlock()
	require.False(t, stillHalted,
		"documents the limitation: one shared bool means batch #2's resume() reopens the "+
			"target even though batch #1 is still failed. If this ever becomes true, the "+
			"breaker was made per-batch -- update the bounded-memory reasoning accordingly.")
}
