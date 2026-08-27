// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

// M5 (swarm major, NOT A BUG -- intentional upstream behavior): on the synchronous path
// (concurrency <= 1, retryHeap == nil) a send interrupted by shutdown calls batch.drop(),
// persisting file offsets for events that never reached CloudWatch.
//
// This looks like the data loss abandon() was introduced to prevent, but it is deliberate:
// commit 2da9c430 ("Update state when batch dropped", #1789) added batch.updateState() to
// exactly this stop path, stating the goal is "to prevent reprocessing the same batch after
// restart" -- upstream chose loss over duplication here. sender_test.go's
// TestSender/StopChannelClosed asserts the state callback DOES run on stop.
//
// Changing it would reverse #1789, so this test pins the intentional behavior instead. If
// the trade is ever revisited, this is the test to flip (with #1789's owner in the loop).
func TestSynchronousShutdownPersistsOffsetsByDesign(t *testing.T) {
	logger := testutil.NewNopLogger()
	var stateRuns, doneRuns atomic.Int32

	svc := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return nil, awserr.New("OperationAbortedException", "forced by test", nil)
	})
	// nil retryHeap => synchronous sleep-retry path.
	s := newSender(logger, svc, NewTargetManager(logger, svc), nil)

	b := newLogEventBatch(Target{Group: "g", Stream: "s"}, nil)
	b.append(newLogEvent(time.Now(), "payload", func() { doneRuns.Add(1) }))
	b.addStateCallback(func() { stateRuns.Add(1) })

	// Interrupt while the sender is parked in its backoff select.
	go func() {
		time.Sleep(75 * time.Millisecond)
		s.Stop()
	}()
	s.Send(b)

	require.Zero(t, doneRuns.Load(),
		"shutdown must never report undelivered events as delivered")
	require.Equal(t, 1, int(stateRuns.Load()),
		"offsets ARE persisted on synchronous shutdown, by #1789's design; flipping this "+
			"assertion means deliberately reversing that trade")
}

// M4 (swarm major): drop() runs updateState() BEFORE resume(). A panic raised by a state
// callback therefore skips resume(), leaving that target's circuit breaker latched forever.
// The per-batch recover added for the retry heap stops one panic from stranding OTHER
// batches, but it does not un-wedge the panicking batch's own target.
func TestPanicInStateCallbackStillClearsCircuitBreaker(t *testing.T) {
	logger := testutil.NewNopLogger()
	var resumed atomic.Int32

	svc := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	})
	workerPool := NewWorkerPool(1)
	defer workerPool.Stop()
	retryHeap := NewRetryHeap(logger)
	defer retryHeap.Stop()
	p := NewRetryHeapProcessor(retryHeap, workerPool, svc, NewTargetManager(logger, svc), logger, nil)

	b := readyBatch("g", nil, func() { panic("state callback boom") })
	b.addResumeCallback(func() { resumed.Add(1) })
	b.expireAfter = time.Now().Add(-time.Hour) // force the expired -> drop() path
	require.NoError(t, retryHeap.Push(b))

	require.NotPanics(t, func() { p.processReadyMessages() })
	require.Equal(t, 1, int(resumed.Load()),
		"a panic in a state callback must still clear the target's circuit breaker, "+
			"otherwise that log group is permanently wedged")
}

// M2 (swarm major, KNOWN LIMITATION -- documented, not fixed): the circuit breaker is a
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

// stubSender is a no-op Sender for breaker-level tests.
type stubSender struct{ sent atomic.Int32 }

func (s *stubSender) Send(*logEventBatch) { s.sent.Add(1) }
func (s *stubSender) Stop()               {}
