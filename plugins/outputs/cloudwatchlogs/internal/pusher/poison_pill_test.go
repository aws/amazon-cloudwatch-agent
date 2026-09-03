// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

// TestRetryHeapProcessorDoesNotStarveAllowedTarget validates that when 10 denied + 1 allowed log groups
// share a worker pool with concurrency=2, the allowed log group continues
// publishing without being starved by failed retries.
// Note: This test pushes batches directly to the heap and bypasses the full
// queue → sender → retryHeap → processor pipeline. It validates RetryHeapProcessor
// behavior, not the end-to-end circuit breaker flow.
func TestRetryHeapProcessorDoesNotStarveAllowedTarget(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Close()

	workerPool := NewWorkerPool(2, &testutil.Logger{}) // Low concurrency as in the bug scenario
	defer workerPool.Stop()

	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("InitTarget", mock.Anything).Return(nil)

	accessDeniedErr := &cloudwatchlogs.AccessDeniedException{
		Message_: aws.String("User is not authorized to perform: logs:PutLogEvents with an explicit deny"),
	}

	// Track successful PutLogEvents calls for the allowed log group
	var allowedGroupSuccessCount atomic.Int32
	var deniedGroupAttemptCount atomic.Int32

	// Configure mock service responses with realistic latency
	mockService.On("PutLogEvents", mock.MatchedBy(func(input *cloudwatchlogs.PutLogEventsInput) bool {
		return *input.LogGroupName == "log-stream-ple-access-granted"
	})).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Run(func(_ mock.Arguments) {
		time.Sleep(10 * time.Millisecond) // Simulate API latency
		allowedGroupSuccessCount.Add(1)
	})

	mockService.On("PutLogEvents", mock.MatchedBy(func(input *cloudwatchlogs.PutLogEventsInput) bool {
		return *input.LogGroupName != "log-stream-ple-access-granted"
	})).Return((*cloudwatchlogs.PutLogEventsOutput)(nil), accessDeniedErr).Run(func(_ mock.Arguments) {
		time.Sleep(10 * time.Millisecond) // Simulate API latency
		deniedGroupAttemptCount.Add(1)
	})

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	// Targets
	allowedTarget := Target{Group: "log-stream-ple-access-granted", Stream: "i-test"}
	deniedTargets := make([]Target, 10)
	for i := 0; i < 10; i++ {
		deniedTargets[i] = Target{
			Group:  "aws-restricted-log-group-name-log-stream-ple-access-denied" + string(rune('0'+i)),
			Stream: "i-test",
		}
	}

	// Simulate continuous batch generation over time (like force_flush_interval=5s)
	done := make(chan struct{})
	var wg sync.WaitGroup

	// Continuously generate batches for denied log groups (simulating continuous log writes)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(target Target) {
			defer wg.Done()
			ticker := time.NewTicker(50 * time.Millisecond) // Simulate flush interval
			defer ticker.Stop()
			batchCount := 0
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					if batchCount >= 5 { // Generate 5 batches per denied log group
						return
					}
					batch := createBatch(target, 50)
					batch.nextRetryTime = time.Now().Add(-1 * time.Second)
					heap.Push(batch)
					batchCount++
				}
			}
		}(deniedTargets[i])
	}

	// Continuously generate batches for allowed log group
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		batchCount := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if batchCount >= 10 { // Generate 10 batches for allowed log group
					return
				}
				batch := createBatch(allowedTarget, 20)
				batch.nextRetryTime = time.Now().Add(-1 * time.Second)
				heap.Push(batch)
				batchCount++
			}
		}
	}()

	// Process batches continuously
	processorDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-processorDone:
				return
			case <-ticker.C:
				processor.processReadyMessages()
			}
		}
	}()

	// Run for 2 seconds to simulate sustained load
	time.Sleep(2 * time.Second)
	close(done)
	wg.Wait()

	// Process remaining messages
	time.Sleep(500 * time.Millisecond)
	processor.processReadyMessages()
	time.Sleep(200 * time.Millisecond)
	close(processorDone)

	// CRITICAL ASSERTION: Allowed log group MUST receive events throughout the test
	successCount := allowedGroupSuccessCount.Load()
	t.Logf("Allowed group success count: %d, Denied group attempt count: %d", successCount, deniedGroupAttemptCount.Load())

	assert.Greater(t, successCount, int32(5),
		"Allowed log group must continue receiving events despite continuous denied log group failures. Got %d, expected > 5", successCount)

	// Verify denied log groups attempted to send
	assert.Greater(t, deniedGroupAttemptCount.Load(), int32(0),
		"Denied log groups should have attempted to send")
}

// TestSingleDeniedLogGroup validates the baseline scenario where a single denied
// log group does not affect the allowed log group.
func TestSingleDeniedLogGroup(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Close()

	workerPool := NewWorkerPool(4, &testutil.Logger{}) // Higher concurrency as in initial test
	defer workerPool.Stop()

	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("InitTarget", mock.Anything).Return(nil)

	accessDeniedErr := &cloudwatchlogs.AccessDeniedException{
		Message_: aws.String("Access denied"),
	}

	var allowedGroupSuccessCount atomic.Int32

	mockService.On("PutLogEvents", mock.MatchedBy(func(input *cloudwatchlogs.PutLogEventsInput) bool {
		return *input.LogGroupName == "log-stream-ple-access-granted"
	})).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Run(func(_ mock.Arguments) {
		allowedGroupSuccessCount.Add(1)
	})

	mockService.On("PutLogEvents", mock.MatchedBy(func(input *cloudwatchlogs.PutLogEventsInput) bool {
		return *input.LogGroupName == "aws-restricted-log-group-name-log-stream-ple-access-denied"
	})).Return((*cloudwatchlogs.PutLogEventsOutput)(nil), accessDeniedErr)

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	// Create batches
	allowedTarget := Target{Group: "log-stream-ple-access-granted", Stream: "i-test"}
	deniedTarget := Target{Group: "aws-restricted-log-group-name-log-stream-ple-access-denied", Stream: "i-test"}

	allowedBatch := createBatch(allowedTarget, 40)
	deniedBatch := createBatch(deniedTarget, 40)

	allowedBatch.nextRetryTime = time.Now().Add(-1 * time.Second)
	deniedBatch.nextRetryTime = time.Now().Add(-1 * time.Second)

	err := heap.Push(allowedBatch)
	assert.NoError(t, err)
	err = heap.Push(deniedBatch)
	assert.NoError(t, err)

	processor.processReadyMessages()

	// Verify allowed log group received events
	require.Eventually(t, func() bool { return allowedGroupSuccessCount.Load() > 0 },
		2*time.Second, 10*time.Millisecond,
		"Allowed log group must receive events with single denied log group")
}

// createBatch creates a log event batch with the specified number of events
func createBatch(target Target, eventCount int) *logEventBatch {
	batch := newLogEventBatch(target, nil)
	batch.events = make([]*cloudwatchlogs.InputLogEvent, eventCount)
	now := time.Now().Unix() * 1000
	for i := 0; i < eventCount; i++ {
		batch.events[i] = &cloudwatchlogs.InputLogEvent{
			Message:   aws.String("test message"),
			Timestamp: aws.Int64(now + int64(i)),
		}
	}
	return batch
}

// Every target failing at once must not deadlock or grow without bound.
func TestAllTargetsFailingDoesNotDeadlock(t *testing.T) {
	logger := &testutil.Logger{}
	var calls atomic.Int32
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		calls.Add(1)
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	})
	workerPool := NewWorkerPool(4, &testutil.Logger{})
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
		retryHeap.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(40 * time.Second):
		t.Fatal("shutdown deadlocked with all targets failing")
	}
}

// Destinations churning while the service throttles must not wedge: the TargetManager's
// shared retryer must outlive individual destination stops.
func TestChurnWhileThrottledDoesNotWedge(t *testing.T) {
	logger := &testutil.Logger{}
	var calls atomic.Int32
	service := okStubService(func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if calls.Add(1)%3 == 0 {
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		}
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	})
	workerPool := NewWorkerPool(4, &testutil.Logger{})
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
		"TargetManager broke after destination churn: the shared retryer was tied to a stopped dest")

	p.Stop()
	workerPool.Stop()
	retryHeap.Close()
}
