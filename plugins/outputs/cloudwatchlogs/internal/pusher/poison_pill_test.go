// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

// TestPoisonPillScenario validates that when 10 denied + 1 allowed log groups
// share a worker pool with concurrency=2, the allowed log group continues
// publishing without being starved by failed retries.
func TestPoisonPillScenario(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(2) // Low concurrency as in the bug scenario
	defer workerPool.Stop()

	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("EnsureTargetExists", mock.Anything).Return(nil)

	accessDeniedErr := &cloudwatchlogs.AccessDeniedException{
		Message_: stringPtr("User is not authorized to perform: logs:PutLogEvents with an explicit deny"),
	}

	// Track successful PutLogEvents calls for the allowed log group
	var allowedGroupSuccessCount atomic.Int32
	var deniedGroupAttemptCount atomic.Int32

	// Configure mock service responses with realistic latency
	mockService.On("PutLogEvents", mock.MatchedBy(func(input *cloudwatchlogs.PutLogEventsInput) bool {
		return *input.LogGroupName == "log-stream-ple-access-granted"
	})).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond) // Simulate API latency
		allowedGroupSuccessCount.Add(1)
	})

	mockService.On("PutLogEvents", mock.MatchedBy(func(input *cloudwatchlogs.PutLogEventsInput) bool {
		return *input.LogGroupName != "log-stream-ple-access-granted"
	})).Return((*cloudwatchlogs.PutLogEventsOutput)(nil), accessDeniedErr).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond) // Simulate API latency
		deniedGroupAttemptCount.Add(1)
	})

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, 100*time.Millisecond, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

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

// TestRetryHeapSmallerThanFailingLogGroups verifies that with an unbounded retry
// heap, the system handles more failing log groups than workers without deadlock.
func TestRetryHeapSmallerThanFailingLogGroups(t *testing.T) {
	concurrency := 2
	numFailingLogGroups := 10

	// Retry heap is now unbounded (maxSize parameter ignored)
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(concurrency)
	defer workerPool.Stop()

	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("EnsureTargetExists", mock.Anything).Return(nil)

	accessDeniedErr := &cloudwatchlogs.AccessDeniedException{
		Message_: stringPtr("Access denied"),
	}

	var allowedGroupSuccessCount atomic.Int32
	var deniedGroupAttemptCount atomic.Int32

	mockService.On("PutLogEvents", mock.MatchedBy(func(input *cloudwatchlogs.PutLogEventsInput) bool {
		return *input.LogGroupName == "allowed"
	})).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
		allowedGroupSuccessCount.Add(1)
	})

	mockService.On("PutLogEvents", mock.MatchedBy(func(input *cloudwatchlogs.PutLogEventsInput) bool {
		return *input.LogGroupName != "allowed"
	})).Return((*cloudwatchlogs.PutLogEventsOutput)(nil), accessDeniedErr).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
		deniedGroupAttemptCount.Add(1)
	})

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, 50*time.Millisecond, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

	// Create targets
	allowedTarget := Target{Group: "allowed", Stream: "stream"}
	deniedTargets := make([]Target, numFailingLogGroups)
	for i := 0; i < numFailingLogGroups; i++ {
		deniedTargets[i] = Target{Group: fmt.Sprintf("denied-%d", i), Stream: "stream"}
	}

	done := make(chan struct{})
	var wg sync.WaitGroup

	// Generate batches for all failing log groups continuously
	for i := 0; i < numFailingLogGroups; i++ {
		wg.Add(1)
		go func(target Target) {
			defer wg.Done()
			ticker := time.NewTicker(30 * time.Millisecond)
			defer ticker.Stop()
			batchCount := 0
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					if batchCount >= 3 {
						return
					}
					batch := createBatch(target, 10)
					batch.nextRetryTime = time.Now().Add(-1 * time.Second)
					heap.Push(batch)
					batchCount++
				}
			}
		}(deniedTargets[i])
	}

	// Generate batches for allowed log group
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(30 * time.Millisecond)
		defer ticker.Stop()
		batchCount := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if batchCount >= 5 {
					return
				}
				batch := createBatch(allowedTarget, 10)
				batch.nextRetryTime = time.Now().Add(-1 * time.Second)
				heap.Push(batch)
				batchCount++
			}
		}
	}()

	// Process continuously
	processorDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Millisecond)
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

	// Run for 1 second
	time.Sleep(1 * time.Second)
	close(done)
	wg.Wait()
	time.Sleep(300 * time.Millisecond)
	processor.processReadyMessages()
	time.Sleep(100 * time.Millisecond)
	close(processorDone)

	successCount := allowedGroupSuccessCount.Load()

	t.Logf("Results: Allowed success=%d, Denied attempts=%d, Heap size=%d, Failing groups=%d",
		successCount, deniedGroupAttemptCount.Load(), heap.Size(), numFailingLogGroups)

	// With unbounded heap, allowed log group should receive events
	assert.Greater(t, successCount, int32(0),
		"Allowed log group must receive events despite %d failing groups", numFailingLogGroups)
}

// TestSingleDeniedLogGroup validates the baseline scenario where a single denied
// log group does not affect the allowed log group.
func TestSingleDeniedLogGroup(t *testing.T) {
	heap := NewRetryHeap(&testutil.Logger{})
	defer heap.Stop()

	workerPool := NewWorkerPool(4) // Higher concurrency as in initial test
	defer workerPool.Stop()

	mockService := &mockLogsService{}
	mockTargetManager := &mockTargetManager{}
	mockTargetManager.On("EnsureTargetExists", mock.Anything).Return(nil)

	accessDeniedErr := &cloudwatchlogs.AccessDeniedException{
		Message_: stringPtr("Access denied"),
	}

	var allowedGroupSuccessCount atomic.Int32

	mockService.On("PutLogEvents", mock.MatchedBy(func(input *cloudwatchlogs.PutLogEventsInput) bool {
		return *input.LogGroupName == "log-stream-ple-access-granted"
	})).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Run(func(args mock.Arguments) {
		allowedGroupSuccessCount.Add(1)
	})

	mockService.On("PutLogEvents", mock.MatchedBy(func(input *cloudwatchlogs.PutLogEventsInput) bool {
		return *input.LogGroupName == "aws-restricted-log-group-name-log-stream-ple-access-denied"
	})).Return((*cloudwatchlogs.PutLogEventsOutput)(nil), accessDeniedErr)

	processor := NewRetryHeapProcessor(heap, workerPool, mockService, mockTargetManager, &testutil.Logger{}, time.Hour, retryer.NewLogThrottleRetryer(&testutil.Logger{}))

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
	time.Sleep(100 * time.Millisecond)

	// Verify allowed log group received events
	assert.Greater(t, allowedGroupSuccessCount.Load(), int32(0),
		"Allowed log group must receive events with single denied log group")
}

// createBatch creates a log event batch with the specified number of events
func createBatch(target Target, eventCount int) *logEventBatch {
	batch := newLogEventBatch(target, nil)
	batch.events = make([]*cloudwatchlogs.InputLogEvent, eventCount)
	now := time.Now().Unix() * 1000
	for i := 0; i < eventCount; i++ {
		batch.events[i] = &cloudwatchlogs.InputLogEvent{
			Message:   stringPtr("test message"),
			Timestamp: int64Ptr(now + int64(i)),
		}
	}
	return batch
}
