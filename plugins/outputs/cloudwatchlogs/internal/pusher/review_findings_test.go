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

func okStubService(ple func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)) *stubLogsService {
	return &stubLogsService{
		ple: ple,
		cls: func(*cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		},
		clg: func(*cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
			return &cloudwatchlogs.CreateLogGroupOutput{}, nil
		},
		dlg: func(*cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
		},
	}
}

// readyBatch returns a batch already past its retry time so PopReady picks it up.
func readyBatch(group string, done, state func()) *logEventBatch {
	b := newLogEventBatch(Target{Group: group, Stream: "stream"}, nil)
	b.append(newLogEvent(time.Now(), "payload", func() {}))
	if done != nil {
		b.addDoneCallback(done)
	}
	if state != nil {
		b.addStateCallback(state)
	}
	b.nextRetryTime = time.Now().Add(-time.Second)
	return b
}

// Finding #4: a batch dropped because the retry heap is already stopped (shutdown in
// progress) must not fire done callbacks. done() runs updateState() AND every
// doneCallback -- per-event LogEvent.Done() plus the queue's onSuccessCallback -- which
// marks never-delivered events as delivered and advances file offsets past them, so the
// events are neither shipped nor re-read after restart.
func TestHeapStoppedDropDoesNotSignalSuccess(t *testing.T) {
	logger := testutil.NewNopLogger()
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	})

	retryHeap := NewRetryHeap(logger)
	retryHeap.Stop() // shutdown already closed the heap

	var delivered, offsetsAdvanced atomic.Int32
	batch := readyBatch("g", func() { delivered.Add(1) }, func() { offsetsAdvanced.Add(1) })

	newSender(logger, service, NewTargetManager(logger, service), retryHeap).Send(batch)

	assert.Zero(t, delivered.Load(),
		"undelivered batch dropped at shutdown fired done callbacks: events are reported delivered")
	assert.Zero(t, offsetsAdvanced.Load(),
		"undelivered batch dropped at shutdown advanced file offsets: events are lost across restart")
}

// Finding #4 (second site): the same false-success signal on the expired-batch drop
// inside flushReadyBatches. Expiry is a permanent give-up, so offsets SHOULD advance --
// but done callbacks must not fire, because nothing was delivered.
func TestExpiredBatchDropDoesNotSignalSuccess(t *testing.T) {
	logger := testutil.NewNopLogger()
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})

	workerPool := NewWorkerPool(1)
	defer workerPool.Stop()
	retryHeap := NewRetryHeap(logger)

	var delivered, offsetsAdvanced atomic.Int32
	batch := readyBatch("g", func() { delivered.Add(1) }, func() { offsetsAdvanced.Add(1) })
	batch.expireAfter = time.Now().Add(-time.Hour) // already past its expiry window
	require.NoError(t, retryHeap.Push(batch))

	p := NewRetryHeapProcessor(retryHeap, workerPool, service, NewTargetManager(logger, service), logger, nil)
	p.flushReadyBatches()

	require.True(t, batch.isExpired(), "test setup: batch must be expired to exercise the drop path")
	assert.Zero(t, delivered.Load(),
		"expired batch fired done callbacks: never-delivered events are reported delivered")
	assert.Equal(t, int32(1), offsetsAdvanced.Load(),
		"expired batch is a permanent give-up, so offsets should advance exactly once")
}

// Finding #2: the circuit breaker halts a target on failure and only clears on
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

// Finding #1: RetryHeapProcessor.Stop() takes stopMu with a deferred Unlock and holds it
// across wg.Wait(). processLoop's ticker path calls processReadyMessages(), which takes
// the same stopMu. If a tick lands while Stop() holds the lock, processLoop never returns
// to its select, never observes stopCh, and never calls wg.Done() -- so Stop() waits
// forever holding the lock processLoop needs.
//
// The window is made deterministic here: Stop() flushes while holding the lock, and the
// flush blocks in workerPool.Submit until the test releases the sends.
func TestRetryHeapProcessorStopDoesNotDeadlock(t *testing.T) {
	logger := testutil.NewNopLogger()

	release := make(chan struct{})
	var inFlight atomic.Int32
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		inFlight.Add(1)
		<-release
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})

	workerPool := NewWorkerPool(1) // 1 worker + task buffer 2 -> Submit blocks on the 4th batch
	retryHeap := NewRetryHeap(logger)

	// Saturate the pool first: the processor's own ticker drains these and parks on release.
	for i := 0; i < 3; i++ {
		require.NoError(t, retryHeap.Push(readyBatch("g", nil, nil)))
	}

	p := NewRetryHeapProcessor(retryHeap, workerPool, service, NewTargetManager(logger, service), logger, nil)
	p.Start()
	require.Eventually(t, func() bool { return inFlight.Load() > 0 }, 5*time.Second, 10*time.Millisecond,
		"test setup: expected the worker pool to be saturated")

	// Now give Stop()'s own flush something to do, so IT is the caller that blocks in
	// Submit while holding stopMu (the tick path releases stopMu before flushing).
	for i := 0; i < 5; i++ {
		require.NoError(t, retryHeap.Push(readyBatch("g", nil, nil)))
	}

	stopped := make(chan struct{})
	go func() { p.Stop(); close(stopped) }()

	// Stop() is now blocked inside flushReadyBatches -> Submit while holding stopMu.
	// Give the 100ms ticker time to fire and park processLoop on the same lock.
	time.Sleep(400 * time.Millisecond)
	close(release)

	select {
	case <-stopped:
	case <-time.After(15 * time.Second):
		t.Fatal("Stop() deadlocked: stopMu is held across wg.Wait() while processLoop's " +
			"ticker path blocks acquiring the same stopMu")
	}
	workerPool.Stop()
}
