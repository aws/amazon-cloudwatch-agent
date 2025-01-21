// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

// CloudWatch Logs PutLogEvents API limits
// Taken from https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
const (
	// The maximum batch size in bytes. This size is calculated as the sum of all event messages in UTF-8,
	// plus 26 bytes for each log event.
	reqSizeLimit = 1024 * 1024
	// The maximum number of log events in a batch.
	reqEventsLimit = 10000
	// The bytes required for metadata for each log event.
	perEventHeaderBytes = 200
	// A batch of log events in a single request cannot span more than 24 hours. Otherwise, the operation fails.
	batchTimeRangeLimit = 24 * time.Hour
)

// logEvent represents a single cloudwatchlogs.InputLogEvent with some metadata for processing
type logEvent struct {
	timestamp    time.Time
	message      string
	eventBytes   int
	doneCallback func()
}

func newLogEvent(timestamp time.Time, message string, doneCallback func()) *logEvent {
	return &logEvent{
		message:      message,
		timestamp:    timestamp,
		eventBytes:   len(message) + perEventHeaderBytes,
		doneCallback: doneCallback,
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
}

func newLogEventBatch(target Target, entityProvider logs.LogEntityProvider) *logEventBatch {
	return &logEventBatch{
		Target:         target,
		events:         make([]*cloudwatchlogs.InputLogEvent, 0),
		entityProvider: entityProvider,
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
	if len(b.events) > 0 && *event.Timestamp < *b.events[len(b.events)-1].Timestamp {
		b.needSort = true
	}
	b.events = append(b.events, event)
	b.addDoneCallback(e.doneCallback)
	b.bufferedSize += e.eventBytes
	if b.minT.IsZero() || b.minT.After(e.timestamp) {
		b.minT = e.timestamp
	}
	if b.maxT.IsZero() || b.maxT.Before(e.timestamp) {
		b.maxT = e.timestamp
	}
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
