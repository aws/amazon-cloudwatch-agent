// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sort"
	"strings"
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
	// The largest counted size (see utf8EncodedLength) of the message of one log event: the agent reserves
	// perEventHeaderBytes of maxEventPayloadBytes for each event.
	maxEventMessageBytes = maxEventPayloadBytes - perEventHeaderBytes
	// The most log events one source log event is split into. Past it, the last of them is truncated and the rest
	// of the message is dropped. With the default max_event_size the most a message from the file tailer needs is 7.
	maxSplitChunks = 8
	// How far, in counted bytes, the cut between two chunks of a split message may move back from the greedy cut to
	// land after a newline, space or tab.
	naturalCutWindow = 4096
)

// invalidUTF8ByteSize is the number of bytes the service counts for each byte of a message that is not valid UTF-8.
// The SDK's JSON encoder writes such a byte as \ufffd, which the service decodes to U+FFFD (utf8.RuneError), 3 bytes
// in UTF-8.
const invalidUTF8ByteSize = len(string(utf8.RuneError))

// validateAndTruncateMessage ensures events don't exceed limit before we send to CloudWatch
//
// The message is measured the way the service counts it (see utf8EncodedLength), not by len(message). It returns the
// message to send and its counted size, so that the message does not need to be measured again, and whether the
// message was truncated. The part of a truncated message after the cut is dropped. It is used for messages that are
// not split (see IsEMFMessage and newStatefulLogEvents); other messages over the limit are split by splitMessage,
// which drops content only past maxSplitChunks.
func validateAndTruncateMessage(message string) (string, int, bool) {
	// The message is scanned once, to count it. No length-based shortcut is needed: a message must be counted anyway
	// to size its event, and a message that fits is returned unchanged here.
	size := utf8EncodedLength(message)
	if size <= maxEventMessageBytes {
		return message, size, false
	}

	// Truncate the message and add a suffix to indicate truncation
	// The cut is on a rune boundary, and the suffix is ASCII, so its counted length is len(defaultTruncationSuffix).
	prefix, prefixSize := truncateToUTF8EncodedLength(message, maxEventMessageBytes-len(defaultTruncationSuffix))
	truncatedMessage := prefix + defaultTruncationSuffix
	return truncatedMessage, prefixSize + len(defaultTruncationSuffix), true
}

// IsEMFMessage reports whether message looks like an embedded metric format (EMF) document. The cloudwatchlogs output
// switches a destination to EMF mode on the first such message, and the pusher does not split such a message when it
// is over the per-event limit: none of its parts would be a valid EMF document, so it is truncated instead.
func IsEMFMessage(message string) bool {
	return strings.HasPrefix(message, "{") && strings.HasSuffix(message, "}") && strings.Contains(message, "\"CloudWatchMetrics\"")
}

// splitMessage cuts message, whose counted size (see utf8EncodedLength) is size, into chunks of at most budget
// counted bytes each, so that each chunk can be sent as a log event of its own and nothing is dropped. It returns the
// chunks, the counted size of each, and whether content was dropped.
//
// The split is greedy: each chunk but the last is the longest prefix of the rest of the message that fits in budget
// (truncateToUTF8EncodedLength), moved back to just after a newline, space or tab if one is close to the cut (see
// preferNaturalCut). Every cut is at a rune boundary of the decoding the service applies, so no chunk splits a rune,
// the chunks concatenate to message byte for byte, and their counted sizes add up to size: utf8EncodedLength decodes
// left to right, and the decoding of the rest of a message after a rune boundary is the rest of its decoding. No
// marker is added to a chunk that loses nothing.
//
// At most maxChunks chunks are returned. If the message needs more, the last chunk is cut to budget less the length
// of defaultTruncationSuffix, the suffix is appended to it, the rest of the message is dropped, and truncated is true.
// budget must be at least utf8.UTFMax, the largest counted size of one rune, so that every chunk is non-empty, plus
// the length of defaultTruncationSuffix if the cap can be reached; maxEventMessageBytes is far larger.
func splitMessage(message string, size, budget, maxChunks int) ([]string, []int, bool) {
	if budget < utf8.UTFMax {
		// A chunk could be empty, and the loop below would not end. The callers never pass such a budget; keep what
		// fits and drop the rest rather than loop.
		prefix, prefixSize := truncateToUTF8EncodedLength(message, budget)
		return []string{prefix}, []int{prefixSize}, prefixSize < size
	}
	var chunks []string
	var sizes []int
	rest, restSize := message, size
	for restSize > budget {
		if len(chunks) >= maxChunks-1 {
			// The cap is reached: truncate the last chunk as validateAndTruncateMessage does, and drop the rest.
			prefix, prefixSize := truncateToUTF8EncodedLength(rest, budget-len(defaultTruncationSuffix))
			return append(chunks, prefix+defaultTruncationSuffix), append(sizes, prefixSize+len(defaultTruncationSuffix)), true
		}
		chunk, chunkSize := truncateToUTF8EncodedLength(rest, budget)
		chunk, chunkSize = preferNaturalCut(chunk, chunkSize)
		chunks, sizes = append(chunks, chunk), append(sizes, chunkSize)
		// chunk ends on a rune boundary, so the rest decodes to the remaining runes and its counted size is the
		// difference.
		rest, restSize = rest[len(chunk):], restSize-chunkSize
	}
	return append(chunks, rest), append(sizes, restSize), false
}

// preferNaturalCut moves the end of chunk, whose counted size is size, back to just after the last newline within
// its final naturalCutWindow counted bytes, or else to just after the last space or tab within them, so that a split
// message is cut between lines or words where possible. It returns chunk unchanged if there is no such character.
// The character stays at the end of the chunk, so the chunks still concatenate to the message.
//
// An ASCII byte is a rune of its own in any decoding (a multi-byte UTF-8 sequence contains no byte below 0x80), so a
// cut after one is a rune boundary, and the counted size of the part after it can be measured on its own.
func preferNaturalCut(chunk string, size int) (string, int) {
	// Each byte counts at least 1, so the final naturalCutWindow counted bytes lie within the final naturalCutWindow
	// raw bytes. The exact counted window is checked below.
	start := max(0, len(chunk)-naturalCutWindow)
	window := chunk[start:]
	for _, separators := range []string{"\n", " \t"} {
		i := strings.LastIndexAny(window, separators)
		if i < 0 {
			continue
		}
		cut := start + i + 1
		if cut == len(chunk) {
			// The chunk already ends with the character.
			return chunk, size
		}
		tailSize := utf8EncodedLength(chunk[cut:])
		// The character itself (1 counted byte) and everything after it must be within the window. An earlier
		// occurrence of the same characters would be further back, so it cannot be within the window either.
		if 1+tailSize > naturalCutWindow {
			continue
		}
		return chunk[:cut], size - tailSize
	}
	return chunk, size
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
	// Slow path: ValidString returned false, so the message holds at least one byte that is not part of a valid
	// UTF-8 sequence. Walk the message, but skip ASCII runs byte-by-byte: an ASCII byte is a rune of its own and
	// counts as 1, so there is no reason to call DecodeRuneInString for it. This keeps the fallback close to the
	// fast path on inputs with sparse invalid bytes (e.g. a long ASCII prefix followed by one bad byte), where the
	// former implementation re-decoded every ASCII byte individually (review comment C2).
	size := 0
	for i := 0; i < len(message); {
		// Fast-skip an ASCII run: every byte below 0x80 counts as 1.
		j := i
		for j < len(message) && message[j] < 0x80 {
			j++
		}
		if j > i {
			size += j - i
			i = j
			continue
		}
		// Non-ASCII byte: decode a (possibly invalid) rune and count it as the service would.
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
	// Whether content of the source message was dropped: the message was over the per-event limit and either looks
	// like EMF, so it was truncated rather than split, or needed more than maxSplitChunks events, so the last of them
	// was truncated. It is set on the last event of the source event only. The batch does not log; the queue reports
	// truncated events.
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

// newStatefulLogEvents returns the log events that carry message, the message of one source log event. A message
// within the per-event limit becomes one event, as with newStatefulLogEvent, and is returned as single (split is nil).
// A larger message is split into up to maxSplitChunks events (see splitMessage) and returned as split (single is nil);
// unless it looks like EMF (see IsEMFMessage), in which case it is truncated as a single event instead.
//
// Exactly one of the returned values is non-nil, so the common single-event fast path does not allocate a slice.
//
// Every event gets the timestamp of the source event. Only the last event carries doneCallback and state: the
// queue sends the events of a split message in order and the last event's batch completes after the others, so the
// source event is acknowledged, and its file offset advanced, only once all of it has been sent.
func newStatefulLogEvents(timestamp time.Time, message string, doneCallback func(), state *logEventState) (single *logEvent, split []*logEvent) {
	size := utf8EncodedLength(message)
	if size <= maxEventMessageBytes {
		// The common case: one event, with the message unchanged. size is reused, so the message is scanned once.
		// Returned via single to skip the 1-element-slice allocation of the former []*logEvent return.
		return &logEvent{
			message:      message,
			timestamp:    timestamp,
			eventBytes:   size + perEventHeaderBytes,
			doneCallback: doneCallback,
			state:        state,
		}, nil
	}
	if IsEMFMessage(message) {
		// Parts of an EMF document are not EMF documents, so splitting would only store invalid JSON fragments and
		// spend requests on them. Keep the truncation.
		return newStatefulLogEvent(timestamp, message, doneCallback, state), nil
	}
	chunks, sizes, truncated := splitMessage(message, size, maxEventMessageBytes, maxSplitChunks)
	events := make([]*logEvent, len(chunks))
	for i, chunk := range chunks {
		events[i] = &logEvent{
			message:    chunk,
			timestamp:  timestamp,
			eventBytes: sizes[i] + perEventHeaderBytes,
		}
	}
	// The last event owns the source event's state and done callback, see above.
	last := events[len(events)-1]
	last.truncated = truncated
	last.doneCallback = doneCallback
	last.state = state
	return nil, events
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
