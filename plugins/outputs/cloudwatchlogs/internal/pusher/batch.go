// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sort"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/constants"
)

// CloudWatch Logs PutLogEvents API limits
// Taken from https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
//
// The public API reference states that a batch is counted as the sum of all event messages in UTF-8 plus 26 bytes per
// event. The per-event limit is understood to be counted the same way, on the message as decoded from the JSON
// request: JSON escaping is not counted (e.g. \n or \" counts as the single character it represents), but each byte
// of the message that is not valid UTF-8 is sent as \ufffd and decoded to U+FFFD, which counts as 3 bytes (see
// utf8EncodedLength). This matches the behaviour reported for other shippers (aws/aws-for-fluent-bit#252,
// aws/aws-for-fluent-bit#683, moby/moby#37986). perEventHeaderBytes intentionally reserves more than 26 bytes per
// event, which keeps batches smaller and bounds memory usage.
const (
	// The maximum batch size in bytes. This size is calculated as the sum of all event messages in UTF-8,
	// plus 26 bytes for each log event.
	reqSizeLimit = 1024 * 1024
	// The maximum number of log events in a batch.
	reqEventsLimit = 10000
	// The bytes required for metadata for each log event
	perEventHeaderBytes = 200
	// Maximum size for individual log events (1MB)
	maxEventPayloadBytes = constants.DefaultMaxEventSize
	// A batch of log events in a single request cannot span more than 24 hours. Otherwise, the operation fails.
	batchTimeRangeLimit = 24 * time.Hour
	// Suffix to indicate that a message has been truncated
	defaultTruncationSuffix = "[Truncated...]"
)

// invalidUTF8ByteSize is the number of bytes the service counts for each byte of a message that is not valid UTF-8.
// The SDK's JSON encoder writes such a byte as \ufffd, which the service decodes to U+FFFD (utf8.RuneError), 3 bytes
// in UTF-8.
const invalidUTF8ByteSize = len(string(utf8.RuneError))

// validateAndTruncateMessage ensures events don't exceed limit before we send to CloudWatch
//
// The message is measured the way the service counts it (see utf8EncodedLength), not by len(message). It returns the
// message to send and its counted size, so that the message does not need to be measured again, and whether the
// message was truncated. The part of a truncated message after the cut is dropped.
func validateAndTruncateMessage(message string) (string, int, bool) {
	maxMessageSize := maxEventPayloadBytes - perEventHeaderBytes

	// The message is scanned once, to count it. No length-based shortcut is needed: a message must be counted anyway
	// to size its event, and a message that fits is returned unchanged here.
	size := utf8EncodedLength(message)
	if size <= maxMessageSize {
		return message, size, false
	}

	// Truncate the message and add a suffix to indicate truncation
	// The cut is on a rune boundary, and the suffix is ASCII, so its counted length is len(defaultTruncationSuffix).
	prefix, prefixSize := truncateToUTF8EncodedLength(message, maxMessageSize-len(defaultTruncationSuffix))
	truncatedMessage := prefix + defaultTruncationSuffix
	return truncatedMessage, prefixSize + len(defaultTruncationSuffix), true
}

// utf8EncodedLength returns the size of message as counted by CloudWatch Logs: the length in bytes of the UTF-8
// string the service decodes from the request. For valid UTF-8 this is len(message), as JSON escaping is not
// counted. Each byte that is not part of a valid UTF-8 sequence reaches the service as U+FFFD, so it counts as
// invalidUTF8ByteSize bytes instead of 1.
func utf8EncodedLength(message string) int {
	// Fast path: valid UTF-8 (including all ASCII) is counted as is.
	if utf8.ValidString(message) {
		return len(message)
	}
	size := 0
	for i := 0; i < len(message); {
		n, counted := decodeRuneSize(message[i:])
		size += counted
		i += n
	}
	return size
}

// truncateToUTF8EncodedLength returns the longest prefix of message whose utf8EncodedLength is at most limit. The
// prefix always ends on a rune boundary, so a valid multi-byte UTF-8 sequence is never split. The utf8EncodedLength of
// the prefix is returned with it, so that the prefix does not need to be measured again.
func truncateToUTF8EncodedLength(message string, limit int) (string, int) {
	// Nothing fits in a limit of 0 or less. A negative limit would also make message[:i] below panic.
	if limit <= 0 {
		return "", 0
	}
	if utf8.ValidString(message) {
		if len(message) <= limit {
			return message, len(message)
		}
		// Back up to the first byte of the rune that crosses the limit.
		i := limit
		for i > 0 && !utf8.RuneStart(message[i]) {
			i--
		}
		// A prefix of valid UTF-8 that ends on a rune boundary is valid UTF-8, so it counts as its length.
		return message[:i], i
	}
	size := 0
	for i := 0; i < len(message); {
		n, counted := decodeRuneSize(message[i:])
		if size+counted > limit {
			return message[:i], size
		}
		size += counted
		i += n
	}
	return message, size
}

// decodeRuneSize returns the length in bytes of the first rune in s and the number of bytes the service counts for
// it. Like the SDK's JSON encoder, it treats each byte that does not start a valid UTF-8 sequence as a rune of its
// own that is replaced with U+FFFD, so the byte counts as invalidUTF8ByteSize bytes.
func decodeRuneSize(s string) (int, int) {
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError && n == 1 {
		return n, invalidUTF8ByteSize
	}
	return n, n
}

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
	// Whether the message was over the per-event limit and so was truncated. The batch does not log; the queue
	// reports truncated events.
	truncated bool
}

func newLogEvent(timestamp time.Time, message string, doneCallback func()) *logEvent {
	return newStatefulLogEvent(timestamp, message, doneCallback, nil)
}

func newStatefulLogEvent(timestamp time.Time, message string, doneCallback func(), state *logEventState) *logEvent {
	// Validate and truncate message if necessary
	validatedMessage, size, truncated := validateAndTruncateMessage(message)

	// eventBytes counts the message as the service does. len(validatedMessage) would undercount invalid UTF-8,
	// which the service counts as 3 bytes (U+FFFD) per invalid byte.
	return &logEvent{
		message:      validatedMessage,
		timestamp:    timestamp,
		eventBytes:   size + perEventHeaderBytes,
		truncated:    truncated,
		doneCallback: doneCallback,
		state:        state,
	}
}

// batch builds a types.InputLogEvent from the timestamp and message stored. Converts the timestamp to
// milliseconds to match the PutLogEvents specifications.
func (e *logEvent) build() types.InputLogEvent {
	return types.InputLogEvent{
		Timestamp: aws.Int64(e.timestamp.UnixMilli()),
		Message:   aws.String(e.message),
	}
}

type logEventBatch struct {
	Target
	events         []types.InputLogEvent
	entityProvider logs.LogEntityProvider
	// Total size of all events in the batch.
	bufferedSize int
	// Whether the events need to be sorted before being sent.
	needSort bool
	// Minimum and maximum timestamps in the batch.
	minT, maxT time.Time
	// Callbacks to execute when batch is successfully sent.
	doneCallbacks []func()
	// Callbacks specifically for updating state
	stateCallbacks []func()
	batchers       map[string]*state.RangeQueueBatcher
}

func newLogEventBatch(target Target, entityProvider logs.LogEntityProvider) *logEventBatch {
	return &logEventBatch{
		Target:         target,
		events:         make([]types.InputLogEvent, 0),
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
	if len(b.events) > 0 && *event.Timestamp < *b.events[len(b.events)-1].Timestamp {
		b.needSort = true
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
		b.addStateCallback(batcher.Done)
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

// addStateCallback adds the callback to the state callbacks list.
// State callbacks are specifically for updating the state file and are executed
// even when a batch fails after exhausting all retry attempts.
func (b *logEventBatch) addStateCallback(callback func()) {
	if callback != nil {
		b.stateCallbacks = append(b.stateCallbacks, callback)
	}
}

// done runs all registered callbacks, including both success callbacks and state callbacks.
func (b *logEventBatch) done() {
	b.updateState()

	for i := len(b.doneCallbacks) - 1; i >= 0; i-- {
		done := b.doneCallbacks[i]
		done()
	}
}

// updateState runs only the state callbacks to update the state file
// without executing other success-related callbacks. This is used when a batch
// fails after exhausting all retry attempts to prevent reprocessing the same
// batch after restart.
func (b *logEventBatch) updateState() {
	for i := len(b.stateCallbacks) - 1; i >= 0; i-- {
		callback := b.stateCallbacks[i]
		callback()
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

type byTimestamp []types.InputLogEvent

func (t byTimestamp) Len() int {
	return len(t)
}

func (t byTimestamp) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t byTimestamp) Less(i, j int) bool {
	return *t[i].Timestamp < *t[j].Timestamp
}
