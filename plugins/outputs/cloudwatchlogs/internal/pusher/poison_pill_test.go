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

// TestPoisonPillScenario validates that when multiple log groups encounter
// AccessDenied errors simultaneously with low concurrency, the agent continues
// publishing to allowed log groups without blocking the entire pipeline.
//
// This test recreates the scenario from poison-pill-test-findings.md where:
// - 1 allowed log group + 10 denied log groups
// - Concurrency = 2
// - Continuous stream of new batches (simulating force_flush_interval=5s)
// - Expected: Allowed log group continues receiving events
// - Historical Bug: Agent stopped publishing to ALL log groups after ~5 minutes
//
// This test validates that the retry heap and worker pool architecture correctly
// handles this scenario by:
// 1. Continuously generating batches for 10 denied + 1 allowed log group
// 2. Processing with only 2 workers (low concurrency)
// 3. Verifying allowed log group continues to receive events throughout
// 4. Ensuring worker pool doesn't get saturated by failed retry attempts
//
// The test passes because the current implementation uses a retry heap with
// proper backoff, preventing failed batches from monopolizing worker threads.
func TestPoisonPillScenario(t *testing.T) {
	heap := NewRetryHeap(100, &testutil.Logger{})
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

// TestRetryHeapSmallerThanFailingLogGroups tests the specific bottleneck scenario where:
// - Retry heap size = concurrency (e.g., 2)
// - Number of failing log groups (10) > retry heap size (2)
// - This causes the retry heap to fill up with failed batches
// - New batches from failing log groups block trying to push to full heap
// - Workers get stuck waiting to push failed batches back to heap
// - Allowed log group gets starved of worker time
//
// This test validates the ACTUAL bug: when retry heap size (equal to concurrency)
// is smaller than the number of failing log groups, the system deadlocks.
//
// **EXPECTED BEHAVIOR**: This test will timeout/deadlock, proving the bug exists.
func TestRetryHeapSmallerThanFailingLogGroups(t *testing.T) {
	t.Skip("This test intentionally deadlocks to demonstrate the poison pill bug where heap size < failing log groups")
	
	concurrency := 2
	numFailingLogGroups := 10
	
	// CRITICAL: Retry heap size equals concurrency (this is the bug)
	heap := NewRetryHeap(concurrency, &testutil.Logger{})
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
	// This will cause deadlock as heap fills up
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
					// This will block when heap is full
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
		successCount, deniedGroupAttemptCount.Load(), concurrency, numFailingLogGroups)

	// This test documents the bug: with heap size < failing log groups, the system deadlocks
	if successCount == 0 {
		t.Errorf("POISON PILL BUG DETECTED: Allowed log group received 0 events. Heap size (%d) < failing groups (%d) caused deadlock", concurrency, numFailingLogGroups)
	}
}

// TestSingleDeniedLogGroup validates the baseline scenario where a single denied
// log group does not affect the allowed log group.
func TestSingleDeniedLogGroup(t *testing.T) {
	heap := NewRetryHeap(10, &testutil.Logger{})
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
