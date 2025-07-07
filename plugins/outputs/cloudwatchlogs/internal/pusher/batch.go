// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

// Helper function for max of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CloudWatch Logs PutLogEvents API limits
// Taken from https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
const (
	// The maximum batch size in bytes. This size is calculated as the sum of all event messages in UTF-8,
	// plus 26 bytes for each log event.
	reqSizeLimit = 1024 * 1024
	// The maximum number of log events in a batch.
	reqEventsLimit = 10000
	// The bytes required for metadata for each log event.
	// Fixed to use the correct 52 bytes as per PutLogEvents API specification (was incorrectly 200)
	perEventHeaderBytes = 0
	// A batch of log events in a single request cannot span more than 24 hours. Otherwise, the operation fails.
	batchTimeRangeLimit = 24 * time.Hour
)

type logEventState struct {
	r     state.Range
	queue state.FileRangeQueue
}

// logEvent represents a single cloudwatchlogs.InputLogEvent with some metadata for processing
type logEvent struct {
	timestamp    time.Time
	message      string
	eventBytes   int
	doneCallback func()
	state        *logEventState
}

func newLogEvent(timestamp time.Time, message string, doneCallback func()) *logEvent {
	return newStatefulLogEvent(timestamp, message, doneCallback, nil)
}

func newStatefulLogEvent(timestamp time.Time, message string, doneCallback func(), state *logEventState) *logEvent {
	return &logEvent{
		message:      message,
		timestamp:    timestamp,
		eventBytes:   len(message) + perEventHeaderBytes,
		doneCallback: doneCallback,
		state:        state,
	}
}

// batch builds a cloudwatchlogs.InputLogEvent from the timestamp and message stored. Converts the timestamp to
// milliseconds to match the PutLogEvents specifications.
func (e *logEvent) build() *cloudwatchlogs.InputLogEvent {
	return &cloudwatchlogs.InputLogEvent{
		Timestamp: aws.Int64(e.timestamp.UnixMilli()),
		Message:   aws.String(e.message),
	}
}

type logEventBatch struct {
	Target
	events         []*cloudwatchlogs.InputLogEvent
	entityProvider logs.LogEntityProvider
	// Total size of all events in the batch.
	bufferedSize int
	// Whether the events need to be sorted before being sent.
	needSort bool
	// Minimum and maximum timestamps in the batch.
	minT, maxT time.Time
	// Callbacks to execute when batch is successfully sent.
	doneCallbacks []func()
	batchers      map[string]*state.RangeQueueBatcher
}

func newLogEventBatch(target Target, entityProvider logs.LogEntityProvider) *logEventBatch {
	return &logEventBatch{
		Target:         target,
		events:         make([]*cloudwatchlogs.InputLogEvent, 0),
		entityProvider: entityProvider,
		batchers:       make(map[string]*state.RangeQueueBatcher),
	}
}

// inTimeRange checks if adding an event with the timestamp would keep the batch within the 24-hour limit.
func (b *logEventBatch) inTimeRange(timestamp time.Time) bool {
	if b.minT.IsZero() || b.maxT.IsZero() {
		return true
	}
	return timestamp.Sub(b.minT) <= batchTimeRangeLimit &&
		b.maxT.Sub(timestamp) <= batchTimeRangeLimit
}

// hasSpace checks if adding an event of the given size will exceed the space limits.
func (b *logEventBatch) hasSpace(size int) bool {
	return len(b.events) < reqEventsLimit && b.bufferedSize+size <= reqSizeLimit
}

// append adds a log event to the batch.
func (b *logEventBatch) append(e *logEvent) {
	event := e.build()

	// Detailed logging for batch processing
	if len(b.events) == 0 {
		// First event in batch
		fmt.Printf("[BATCH DEBUG] Starting new batch for Log Group: %s, Stream: %s\n", b.Group, b.Stream)
	}

	fmt.Printf("[BATCH DEBUG] Adding event to batch:")
	fmt.Printf("  - Log Group: %s\n", b.Group)
	fmt.Printf("  - Event #%d in batch\n", len(b.events)+1)
	fmt.Printf("  - Event size: %d bytes (message: %d + header: %d)\n", e.eventBytes, len(e.message), perEventHeaderBytes)
	fmt.Printf("  - Current batch size: %d bytes\n", b.bufferedSize)
	fmt.Printf("  - New batch size will be: %d bytes\n", b.bufferedSize+e.eventBytes)
	fmt.Printf("  - Batch size limit: %d bytes\n", reqSizeLimit)
	fmt.Printf("  - Events in batch: %d (limit: %d)\n", len(b.events), reqEventsLimit)

	// Check if message appears truncated
	if len(e.message) >= 15 && e.message[len(e.message)-15:] == "[Truncated...]" {
		fmt.Printf("  - ⚠️  TRUNCATED MESSAGE DETECTED in batch\n")
		fmt.Printf("  - Message preview (last 100 chars): %s\n", e.message[max(0, len(e.message)-100):])
	}

	// Warn if batch is getting close to limits
	if b.bufferedSize+e.eventBytes > reqSizeLimit*4/5 { // 80% of limit
		fmt.Printf("  - ⚠️  Batch approaching size limit (>80%%): %d/%d bytes\n", b.bufferedSize+e.eventBytes, reqSizeLimit)
	}

	if len(b.events)+1 > reqEventsLimit*4/5 { // 80% of limit
		fmt.Printf("  - ⚠️  Batch approaching event count limit (>80%%): %d/%d events\n", len(b.events)+1, reqEventsLimit)
	}

	if len(b.events) > 0 && *event.Timestamp < *b.events[len(b.events)-1].Timestamp {
		b.needSort = true
		fmt.Printf("  - ⚠️  Event timestamp out of order, will need sorting\n")
	}

	b.events = append(b.events, event)
	// do not add done callback for stateful log events. each batcher will add its own callback
	if e.state != nil && e.state.queue != nil {
		b.handleLogEventState(e.state)
	} else {
		b.addDoneCallback(e.doneCallback)
	}
	b.bufferedSize += e.eventBytes
	if b.minT.IsZero() || b.minT.After(e.timestamp) {
		b.minT = e.timestamp
	}
	if b.maxT.IsZero() || b.maxT.Before(e.timestamp) {
		b.maxT = e.timestamp
	}
}

func (b *logEventBatch) handleLogEventState(s *logEventState) {
	queueID := s.queue.ID()
	batcher, ok := b.batchers[queueID]
	if !ok {
		batcher = state.NewRangeQueueBatcher(s.queue)
		b.addDoneCallback(batcher.Done)
		b.batchers[queueID] = batcher
	}
	batcher.Merge(s.r)
}

// addDoneCallback adds the callback to the end of the registered callbacks.
func (b *logEventBatch) addDoneCallback(callback func()) {
	if callback != nil {
		b.doneCallbacks = append(b.doneCallbacks, callback)
	}
}

// done runs all registered callbacks.
func (b *logEventBatch) done() {
	for i := len(b.doneCallbacks) - 1; i >= 0; i-- {
		done := b.doneCallbacks[i]
		done()
	}
}

// build creates a cloudwatchlogs.PutLogEventsInput from the batch. The log events in the batch must be in
// chronological order by their timestamp.
func (b *logEventBatch) build() *cloudwatchlogs.PutLogEventsInput {
	if b.needSort {
		sort.Stable(byTimestamp(b.events))
	}
	input := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(b.Group),
		LogStreamName: aws.String(b.Stream),
		LogEvents:     b.events,
	}
	if b.entityProvider != nil {
		input.Entity = b.entityProvider.Entity()
	}
	return input
}

type byTimestamp []*cloudwatchlogs.InputLogEvent

func (t byTimestamp) Len() int {
	return len(t)
}

func (t byTimestamp) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t byTimestamp) Less(i, j int) bool {
	return *t[i].Timestamp < *t[j].Timestamp
}
