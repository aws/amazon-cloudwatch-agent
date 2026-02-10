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
	retryHeap := NewRetryHeap(concurrency, logger)
	defer workerPool.Stop()
	defer retryHeap.Stop()

	tm := NewTargetManager(logger, service)

	var wg sync.WaitGroup
	flushTimeout := 50 * time.Millisecond

	failingPusher := NewPusher(logger, failingTarget, service, tm, nil, workerPool, flushTimeout, &wg, 2, retryHeap)
	healthyPusher := NewPusher(logger, healthyTarget, service, tm, nil, workerPool, flushTimeout, &wg, 2, retryHeap)
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
	assert.LessOrEqual(t, failingTargetSendCount.Load(), int32(1),
		"Circuit breaker should block failing target from sending more than 1 batch, "+
			"but %d batches were sent. Without a circuit breaker, the failing target "+
			"continues flooding the worker pool with bad requests.", failingTargetSendCount.Load())

	// Healthy target should continue sending successfully
	assert.Greater(t, healthyTargetSendCount.Load(), int32(0),
		"Healthy target should continue sending while failing target is blocked")
}
