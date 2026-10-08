// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	smithyjson "github.com/aws/smithy-go/encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/logs"
)

type mockEntityProvider struct {
	mock.Mock
}

var _ logs.LogEntityProvider = (*mockEntityProvider)(nil)

func (m *mockEntityProvider) Entity() *types.Entity {
	args := m.Called()
	return args.Get(0).(*types.Entity)
}

func newMockEntityProvider(entity *types.Entity) *mockEntityProvider {
	ep := new(mockEntityProvider)
	ep.On("Entity").Return(entity)
	return ep
}

type mockDoneCallback struct {
	mock.Mock
}

func (m *mockDoneCallback) Done() {
	m.Called()
}

func TestLogEvent(t *testing.T) {
	now := time.Now()
	e := newLogEvent(now, "test message", nil)
	inputLogEvent := e.build()
	assert.EqualValues(t, now.UnixMilli(), *inputLogEvent.Timestamp)
	assert.EqualValues(t, "test message", *inputLogEvent.Message)
}

func TestLogEventBatch(t *testing.T) {
	t.Run("UpdateStateOnly", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		successCallbackCalled := false
		successCallback := func() {
			successCallbackCalled = true
		}

		stateCallbackCalled := false
		stateCallback := func() {
			stateCallbackCalled = true
		}

		batch.addDoneCallback(successCallback)
		batch.addStateCallback(stateCallback)

		batch.updateState()

		assert.False(t, successCallbackCalled, "Success callback should not have been called")
		assert.True(t, stateCallbackCalled, "State callback should have been called")
	})

	t.Run("UpdateStateOnly_WithMultipleCallbacks", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		successCallbacksCalled := make([]bool, 3)
		successCallbacks := []func(){
			func() { successCallbacksCalled[0] = true },
			func() { successCallbacksCalled[1] = true },
			func() { successCallbacksCalled[2] = true },
		}

		stateCallbacksCalled := make([]bool, 3)
		stateCallbacks := []func(){
			func() { stateCallbacksCalled[0] = true },
			func() { stateCallbacksCalled[1] = true },
			func() { stateCallbacksCalled[2] = true },
		}

		for _, cb := range successCallbacks {
			batch.addDoneCallback(cb)
		}
		for _, cb := range stateCallbacks {
			batch.addStateCallback(cb)
		}

		batch.updateState()

		// Verify none of the success callbacks were called
		for i, called := range successCallbacksCalled {
			assert.False(t, called, "Success callback %d should not have been called", i)
		}

		// Verify all state callbacks were called
		for i, called := range stateCallbacksCalled {
			assert.True(t, called, "State callback %d should have been called", i)
		}
	})

	t.Run("UpdateStateOnly_WithRangeQueueBatcher", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		mrq1 := &mockRangeQueue{}
		mrq1.On("ID").Return("test1")
		mrq1.On("Enqueue", state.NewRange(10, 20)).Once()

		mrq2 := &mockRangeQueue{}
		mrq2.On("ID").Return("test2")
		mrq2.On("Enqueue", state.NewRange(30, 40)).Once()

		event1 := newStatefulLogEvent(time.Now(), "Test1", nil, &logEventState{
			r:     state.NewRange(10, 20),
			queue: mrq1,
		})
		event2 := newStatefulLogEvent(time.Now(), "Test2", nil, &logEventState{
			r:     state.NewRange(30, 40),
			queue: mrq2,
		})

		successCallbackCalled := false
		batch.addDoneCallback(func() {
			successCallbackCalled = true
		})

		batch.append(event1)
		batch.append(event2)

		batch.updateState()

		mrq1.AssertExpectations(t)
		mrq2.AssertExpectations(t)

		assert.False(t, successCallbackCalled, "Success callback should not have been called")
	})

	t.Run("UpdateStateWithDone", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		mrq1 := &mockRangeQueue{}
		mrq1.On("ID").Return("test1")
		mrq1.On("Enqueue", state.NewRange(10, 20)).Once()

		mrq2 := &mockRangeQueue{}
		mrq2.On("ID").Return("test2")
		mrq2.On("Enqueue", state.NewRange(30, 40)).Once()

		event1 := newStatefulLogEvent(time.Now(), "Test1", nil, &logEventState{
			r:     state.NewRange(10, 20),
			queue: mrq1,
		})
		event2 := newStatefulLogEvent(time.Now(), "Test2", nil, &logEventState{
			r:     state.NewRange(30, 40),
			queue: mrq2,
		})

		stateCallbackCalled := false
		batch.addStateCallback(func() {
			stateCallbackCalled = true
		})

		batch.append(event1)
		batch.append(event2)

		batch.done()

		mrq1.AssertExpectations(t)
		mrq2.AssertExpectations(t)

		assert.True(t, stateCallbackCalled, "State callback should have been called")
	})

	t.Run("Append", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		event1 := newLogEvent(time.Now(), "Test message 1", nil)
		event2 := newLogEvent(time.Now(), "Test message 2", nil)

		batch.append(event1)
		assert.Equal(t, 1, len(batch.events), "Batch should have 1 event")

		batch.append(event2)
		assert.Equal(t, 2, len(batch.events), "Batch should have 2 events")
	})

	t.Run("InTimeRange", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		now := time.Now()
		assert.True(t, batch.inTimeRange(now))
		event1 := newLogEvent(now, "Test message 1", nil)
		batch.append(event1)

		assert.True(t, batch.inTimeRange(now.Add(23*time.Hour)), "Time within 24 hours should be in range")
		assert.False(t, batch.inTimeRange(now.Add(25*time.Hour)), "Time beyond 24 hours should not be in range")
		assert.False(t, batch.inTimeRange(now.Add(-25*time.Hour)), "Time more than 24 hours in past should not be in range")
	})

	t.Run("HasSpace", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		// Test with empty batch
		assert.True(t, batch.hasSpace(reqSizeLimit))
		assert.False(t, batch.hasSpace(reqSizeLimit+1))

		// Add a small event
		smallEvent := newLogEvent(time.Now(), "a", nil)
		batch.append(smallEvent)

		// Test with batch containing one small event
		remainingSpace := reqSizeLimit - smallEvent.eventBytes
		assert.True(t, batch.hasSpace(remainingSpace))
		assert.False(t, batch.hasSpace(remainingSpace+1))
	})

	t.Run("Build", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		event1 := newLogEvent(time.Now(), "Test message 1", nil)
		event2 := newLogEvent(time.Now(), "Test message 2", nil)
		batch.append(event1)
		batch.append(event2)

		input := batch.build()

		assert.Equal(t, "G", *input.LogGroupName, "Log group name should match")
		assert.Equal(t, "S", *input.LogStreamName, "Log stream name should match")
		assert.Equal(t, 2, len(input.LogEvents), "Input should have 2 log events")
	})

	t.Run("EventSort", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		now := time.Now()
		event1 := newLogEvent(now.Add(1*time.Second), "Test message 1", nil)
		event2 := newLogEvent(now, "Test message 2", nil)
		event3 := newLogEvent(now.Add(2*time.Second), "Test message 3", nil)

		// Add events in non-chronological order
		batch.append(event1)
		batch.append(event2)
		batch.append(event3)

		input := batch.build()

		assert.Equal(t, 3, len(input.LogEvents), "Input should have 3 log events")
		assert.True(t, *input.LogEvents[0].Timestamp < *input.LogEvents[1].Timestamp, "Events should be sorted by timestamp")
		assert.True(t, *input.LogEvents[1].Timestamp < *input.LogEvents[2].Timestamp, "Events should be sorted by timestamp")
	})

	t.Run("DoneCallback", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		callbackCalled := false
		callback := func() {
			callbackCalled = true
		}

		event := newLogEvent(time.Now(), "Test message", callback)
		batch.append(event)

		batch.done()

		assert.True(t, callbackCalled, "Done callback should have been called")
	})

	t.Run("WithEntityProvider", func(t *testing.T) {
		testEntity := &types.Entity{
			Attributes: map[string]string{
				"PlatformType":         "AWS::EC2",
				"EC2.InstanceId":       "i-123456789",
				"EC2.AutoScalingGroup": "test-group",
			},
			KeyAttributes: map[string]string{
				"Name":         "myService",
				"Environment":  "myEnvironment",
				"AwsAccountId": "123456789",
			},
		}
		mockProvider := newMockEntityProvider(testEntity)
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, mockProvider)

		event := newLogEvent(time.Now(), "Test message", nil)
		batch.append(event)

		input := batch.build()

		assert.Equal(t, testEntity, input.Entity, "Entity should be set from the EntityProvider")
	})

	t.Run("WithStatefulLogEvents", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		mdc1 := &mockDoneCallback{}
		mdc1.On("Done").Panic("should not be called")

		mrq1 := &mockRangeQueue{}
		mrq1.On("ID").Return("test")
		mrq1.On("Enqueue", state.NewRange(20, 50)).Once()

		mrq2 := &mockRangeQueue{}
		mrq2.On("ID").Return("test2")
		mrq2.On("Enqueue", state.NewRange(5, 20)).Once()

		event1 := newStatefulLogEvent(time.Now(), "Test", mdc1.Done, &logEventState{
			r:     state.NewRange(20, 40),
			queue: mrq1,
		})
		event2 := newStatefulLogEvent(time.Now(), "Test2", mdc1.Done, &logEventState{
			r:     state.NewRange(5, 20),
			queue: mrq2,
		})
		event3 := newStatefulLogEvent(time.Now(), "Test3", mdc1.Done, &logEventState{
			r:     state.NewRange(40, 50),
			queue: mrq1,
		})

		mdc2 := &mockDoneCallback{}
		mdc2.On("Done").Return().Once()
		event4 := newLogEvent(time.Now(), "Test2", mdc2.Done)
		batch.append(event1)
		batch.append(event2)
		batch.append(event3)
		batch.append(event4)
		batch.done()

		mrq1.AssertExpectations(t)
		mrq2.AssertExpectations(t)
		mdc1.AssertNotCalled(t, "Done")
		mdc2.AssertExpectations(t)
	})
}

func TestEventValidation_1MB(t *testing.T) {
	// Test event at exactly the validation limit
	maxMessageSize := maxEventPayloadBytes - perEventHeaderBytes
	largeMessage := strings.Repeat("a", maxMessageSize)

	event := newStatefulLogEvent(time.Now(), largeMessage, nil, nil)
	assert.Equal(t, largeMessage, event.message)
	assert.Equal(t, maxMessageSize+perEventHeaderBytes, event.eventBytes)
	assert.False(t, event.truncated)
}

func TestEventValidation_Over1MB(t *testing.T) {
	// Test event over 1MB - should be truncated with truncation suffix
	maxMessageSize := maxEventPayloadBytes - perEventHeaderBytes
	oversizeMessage := strings.Repeat("a", maxEventPayloadBytes+1000)

	event := newStatefulLogEvent(time.Now(), oversizeMessage, nil, nil)
	// The total length should still be maxMessageSize
	assert.Equal(t, maxMessageSize, len(event.message))
	assert.Equal(t, oversizeMessage[:maxMessageSize-len(defaultTruncationSuffix)]+defaultTruncationSuffix, event.message)
	assert.True(t, event.truncated)
}

func TestEventValidation_Between256KBand1MB(t *testing.T) {
	// Test event between 256KB and 1MB - should pass through unchanged
	mediumMessage := strings.Repeat("a", 512*1024) // 512KB

	event := newStatefulLogEvent(time.Now(), mediumMessage, nil, nil)
	assert.Equal(t, mediumMessage, event.message)
}

func TestValidateAndTruncateMessage(t *testing.T) {
	maxMessageSize := maxEventPayloadBytes - perEventHeaderBytes

	tests := []struct {
		name              string
		input             string
		expectedOutput    string
		expectedTruncated bool
	}{
		{
			name:           "Small message",
			input:          "small message",
			expectedOutput: "small message",
		},
		{
			name:           "Exactly at limit",
			input:          strings.Repeat("a", maxMessageSize),
			expectedOutput: strings.Repeat("a", maxMessageSize),
		},
		{
			name:              "Over limit",
			input:             strings.Repeat("a", maxMessageSize+1000),
			expectedOutput:    strings.Repeat("a", maxMessageSize-len(defaultTruncationSuffix)) + defaultTruncationSuffix,
			expectedTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, truncated := validateAndTruncateMessage(tt.input)
			assert.Equal(t, tt.expectedOutput, result)
			assert.Equal(t, tt.expectedTruncated, truncated)
		})
	}
}

// TestValidateAndTruncateMessage_Truncated checks that a message is reported as truncated exactly when its counted
// size is over the limit, whatever its raw length, and that the size returned is that of the truncated message.
func TestValidateAndTruncateMessage_Truncated(t *testing.T) {
	maxMessageSize := maxEventPayloadBytes - perEventHeaderBytes
	// Fewer raw bytes than the limit, but counted as invalidUTF8ByteSize bytes each, so over the limit.
	invalidUnderRawLimit := string(bytes.Repeat([]byte{0xff}, maxMessageSize/invalidUTF8ByteSize+1))
	require.Less(t, len(invalidUnderRawLimit), maxMessageSize)
	require.Greater(t, utf8EncodedLength(invalidUnderRawLimit), maxMessageSize)

	tests := []struct {
		name              string
		input             string
		expectedTruncated bool
	}{
		{name: "ASCII exactly at limit", input: strings.Repeat("a", maxMessageSize)},
		{name: "ASCII over limit", input: strings.Repeat("a", maxMessageSize+1), expectedTruncated: true},
		{name: "Multi-byte UTF-8 over limit", input: strings.Repeat("日", maxMessageSize/len("日")+1), expectedTruncated: true},
		{name: "Invalid UTF-8 under raw limit but counted over limit", input: invalidUnderRawLimit, expectedTruncated: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, size, truncated := validateAndTruncateMessage(tt.input)
			assert.Equal(t, tt.expectedTruncated, truncated)
			assert.Equal(t, utf8EncodedLength(result), size)
			assert.LessOrEqual(t, size, maxMessageSize)
			if tt.expectedTruncated {
				assert.True(t, strings.HasSuffix(result, defaultTruncationSuffix))
				assert.Greater(t, utf8EncodedLength(tt.input), size)
			} else {
				assert.Equal(t, tt.input, result)
				assert.Equal(t, maxMessageSize, size)
			}

			event := newLogEvent(time.Now(), tt.input, nil)
			assert.Equal(t, tt.expectedTruncated, event.truncated)
			assert.Equal(t, size+perEventHeaderBytes, event.eventBytes)
		})
	}
}

// serviceEventOverheadBytes is the number of bytes the service counts for each log event in addition to its
// message, per the PutLogEvents API reference.
const serviceEventOverheadBytes = 26

// decodeSerializedMessage returns the message as the service receives it: the message is encoded with the JSON
// encoder the SDK uses to serialize PutLogEvents requests, then decoded.
func decodeSerializedMessage(t *testing.T, message string) string {
	t.Helper()
	encoder := smithyjson.NewEncoder()
	encoder.Value.String(message)
	var decoded string
	require.NoError(t, json.Unmarshal(encoder.Bytes(), &decoded))
	return decoded
}

// serviceCountedSize returns the size of the events as counted by the service: the length in UTF-8 of each decoded
// message plus serviceEventOverheadBytes for each event.
func serviceCountedSize(t *testing.T, events []types.InputLogEvent) int {
	t.Helper()
	size := 0
	for _, event := range events {
		size += len(decodeSerializedMessage(t, *event.Message)) + serviceEventOverheadBytes
	}
	return size
}

// utf8EncodedLengthTestCases pairs messages with the size the service counts for them.
var utf8EncodedLengthTestCases = []struct {
	name     string
	message  string
	expected int
}{
	{name: "Empty", message: "", expected: 0},
	{name: "ASCII", message: "hello world", expected: 11},
	// JSON escaping is not counted: \", \\, \n, \t and \u001b each count as 1 byte.
	{name: "CharactersEscapedInJSON", message: "\"\\\n\t\x1b", expected: 5},
	{name: "TwoByteUTF8", message: "héllo", expected: 6},
	{name: "ThreeByteUTF8", message: "日本語", expected: 9},
	{name: "FourByteUTF8", message: "😀", expected: 4},
	// Escaped by the encoder as \u2028, which decodes back to the same 3 bytes.
	{name: "LineSeparator", message: "\u2028", expected: 3},
	// A valid U+FFFD in the message is not inflated.
	{name: "ValidReplacementCharacter", message: "\uFFFD", expected: 3},
	// Each byte that is not valid UTF-8 is decoded by the service as U+FFFD, 3 bytes.
	{name: "InvalidByte", message: "\xff", expected: 3},
	{name: "Latin1", message: "caf\xe9", expected: 6},
	{name: "LoneContinuationByte", message: "\x80", expected: 3},
	{name: "TruncatedMultiByteSequence", message: "\xe6\x97", expected: 6},
	{name: "OverlongEncoding", message: "\xc0\xaf", expected: 6},
	{name: "EncodedSurrogate", message: "\xed\xa0\x80", expected: 9},
	{name: "MixedValidAndInvalid", message: "a\xffé\xe6\x97日", expected: 15},
}

func TestUTF8EncodedLength(t *testing.T) {
	for _, tc := range utf8EncodedLengthTestCases {
		t.Run(tc.name, func(t *testing.T) {
			got := utf8EncodedLength(tc.message)
			assert.Equal(t, tc.expected, got)
			if utf8.ValidString(tc.message) {
				assert.Equal(t, len(tc.message), got, "valid UTF-8 is counted by its length in bytes")
			} else {
				assert.Greater(t, got, len(tc.message), "invalid UTF-8 is counted as more than its length in bytes")
			}
		})
	}

	t.Run("CountsBytesNotRunes", func(t *testing.T) {
		for _, message := range []string{"héllo", "日本語", "😀"} {
			assert.Equal(t, len(message), utf8EncodedLength(message))
			assert.NotEqual(t, utf8.RuneCountInString(message), utf8EncodedLength(message))
		}
	})
}

// TestUTF8EncodedLength_MatchesSerializedMessage checks that utf8EncodedLength equals the length in UTF-8 of the
// message the service decodes from a request serialized by the SDK.
func TestUTF8EncodedLength_MatchesSerializedMessage(t *testing.T) {
	for _, tc := range utf8EncodedLengthTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// json.Unmarshal would itself replace raw invalid UTF-8 with U+FFFD, so also check the bytes the encoder
			// writes: they are valid UTF-8, and invalid UTF-8 in the message is escaped as \ufffd.
			enc := smithyjson.NewEncoder()
			enc.Value.String(tc.message)
			assert.True(t, utf8.Valid(enc.Bytes()))
			if !utf8.ValidString(tc.message) {
				assert.Contains(t, string(enc.Bytes()), `\ufffd`)
			}
			assert.Equal(t, len(decodeSerializedMessage(t, tc.message)), utf8EncodedLength(tc.message))
		})
	}
}

func TestLogEvent_EventBytes(t *testing.T) {
	t.Run("InvalidUTF8", func(t *testing.T) {
		const invalidBytes = 1000
		// ISO-8859-1 encoded 'é', which is not valid UTF-8.
		message := strings.Repeat("\xe9", invalidBytes)

		event := newLogEvent(time.Now(), message, nil)
		assert.Equal(t, message, event.message)
		assert.Equal(t, invalidBytes*utf8.RuneLen(utf8.RuneError)+perEventHeaderBytes, event.eventBytes)

		// Sizing by the raw length, as before, undercounts by 2 bytes per invalid byte.
		rawEventBytes := len(message) + perEventHeaderBytes
		assert.Equal(t, invalidBytes+perEventHeaderBytes, rawEventBytes)
		assert.Equal(t, 2*invalidBytes, event.eventBytes-rawEventBytes)
	})

	t.Run("ValidMultiByteUTF8", func(t *testing.T) {
		message := strings.Repeat("héllo 日本語 😀 ", 100)

		event := newLogEvent(time.Now(), message, nil)
		assert.Equal(t, message, event.message)
		assert.Equal(t, len(message)+perEventHeaderBytes, event.eventBytes)
	})
}

// TestLogEventBatch_HasSpaceWithInvalidUTF8 fills batches the way the queue does and checks the size of the
// resulting request as the service counts it.
func TestLogEventBatch_HasSpaceWithInvalidUTF8(t *testing.T) {
	const validBytes, invalidBytes = 600, 400
	// A message where some of the bytes are not valid UTF-8, e.g. ISO-8859-1 encoded text.
	message := strings.Repeat("a", validBytes) + strings.Repeat("\xe9", invalidBytes)

	event := newLogEvent(time.Now(), message, nil)
	require.Equal(t, validBytes+invalidBytes*utf8.RuneLen(utf8.RuneError)+perEventHeaderBytes, event.eventBytes)
	// The same event sized by its raw length, as before.
	rawSizedEvent := &logEvent{timestamp: event.timestamp, message: message, eventBytes: len(message) + perEventHeaderBytes}

	// Append events until the batch has no space for the next one.
	fill := func(e *logEvent) *logEventBatch {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		for batch.hasSpace(e.eventBytes) {
			batch.append(e)
		}
		return batch
	}
	batch := fill(event)
	rawSizedBatch := fill(rawSizedEvent)
	require.Len(t, batch.events, reqSizeLimit/event.eventBytes)
	require.Len(t, rawSizedBatch.events, reqSizeLimit/rawSizedEvent.eventBytes)
	require.Greater(t, len(rawSizedBatch.events), len(batch.events))

	// The full batch has no space for another event, while raw-length sizing reported space at the same point.
	assert.False(t, batch.hasSpace(event.eventBytes))
	partialRawSizedBatch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
	for range batch.events {
		partialRawSizedBatch.append(rawSizedEvent)
	}
	assert.True(t, partialRawSizedBatch.hasSpace(rawSizedEvent.eventBytes))

	// Only the batch sized by the decoded UTF-8 length is within the limit enforced by the service.
	assert.LessOrEqual(t, serviceCountedSize(t, batch.events), reqSizeLimit)
	assert.Greater(t, serviceCountedSize(t, rawSizedBatch.events), reqSizeLimit)
}

func TestValidateAndTruncateMessage_UTF8(t *testing.T) {
	maxMessageSize := maxEventPayloadBytes - perEventHeaderBytes
	// The space left for the kept part of a truncated message.
	budget := maxMessageSize - len(defaultTruncationSuffix)
	replacementCharBytes := utf8.RuneLen(utf8.RuneError)
	// The longest run of invalid bytes that fits leaves maxMessageSize%replacementCharBytes bytes of the limit, which
	// "aa" fills exactly. The run plus "aa" is longer than maxMessageSize/invalidUTF8ByteSize bytes, and its counted
	// size is compared to the limit.
	invalidRun := strings.Repeat("\xe9", maxMessageSize/replacementCharBytes)
	require.Equal(t, maxMessageSize, len(invalidRun)*replacementCharBytes+len("aa"))

	tests := []struct {
		name           string
		input          string
		expectedOutput string
		// Whether the kept part of the output must be valid UTF-8.
		validOutput bool
	}{
		{
			name:           "Invalid UTF-8 at limit",
			input:          strings.Repeat("\xe9", maxMessageSize/replacementCharBytes),
			expectedOutput: strings.Repeat("\xe9", maxMessageSize/replacementCharBytes),
		},
		{
			// Counted as exactly maxMessageSize bytes, see invalidRun.
			name:           "Invalid UTF-8 with ASCII tail exactly at limit",
			input:          invalidRun + "aa",
			expectedOutput: invalidRun + "aa",
		},
		{
			// Counted as maxMessageSize+1 bytes.
			name:           "Invalid UTF-8 with ASCII tail one byte over limit",
			input:          invalidRun + "aaa",
			expectedOutput: strings.Repeat("\xe9", budget/replacementCharBytes) + defaultTruncationSuffix,
		},
		{
			name:           "Invalid UTF-8 over limit",
			input:          strings.Repeat("\xe9", maxMessageSize/replacementCharBytes+1),
			expectedOutput: strings.Repeat("\xe9", budget/replacementCharBytes) + defaultTruncationSuffix,
		},
		{
			// Previously passed through untruncated because len(input) is within the limit.
			name:           "Invalid UTF-8 at raw length limit",
			input:          strings.Repeat("\xe9", maxMessageSize),
			expectedOutput: strings.Repeat("\xe9", budget/replacementCharBytes) + defaultTruncationSuffix,
		},
		{
			name:           "Invalid UTF-8 over raw length limit",
			input:          strings.Repeat("\xe9", maxEventPayloadBytes+1000),
			expectedOutput: strings.Repeat("\xe9", budget/replacementCharBytes) + defaultTruncationSuffix,
		},
		{
			// Previously passed through untruncated because len(input) is exactly at the limit.
			name:           "Valid prefix with invalid UTF-8 tail",
			input:          strings.Repeat("a", maxMessageSize-10) + strings.Repeat("\xe9", 10),
			expectedOutput: strings.Repeat("a", budget) + defaultTruncationSuffix,
			validOutput:    true,
		},
		{
			// The invalid byte at the cut point would count as more than the 1 byte of budget left, so it is dropped.
			name:           "Invalid UTF-8 byte at cut point",
			input:          strings.Repeat("a", budget-1) + strings.Repeat("\xe9", 10),
			expectedOutput: strings.Repeat("a", budget-1) + defaultTruncationSuffix,
			validOutput:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, size, truncated := validateAndTruncateMessage(tt.input)
			assert.Equal(t, tt.expectedOutput, result)
			assert.Equal(t, utf8EncodedLength(result), size)
			assert.LessOrEqual(t, utf8EncodedLength(result), maxMessageSize)
			assert.Equal(t, result != tt.input, truncated)
			if result != tt.input {
				require.True(t, strings.HasSuffix(result, defaultTruncationSuffix))
				kept := strings.TrimSuffix(result, defaultTruncationSuffix)
				assert.True(t, strings.HasPrefix(tt.input, kept))
				if tt.validOutput {
					assert.True(t, utf8.ValidString(kept))
				}
			}

			event := newLogEvent(time.Now(), tt.input, nil)
			assert.Equal(t, result, event.message)
			assert.Equal(t, size+perEventHeaderBytes, event.eventBytes)
			assert.Equal(t, truncated, event.truncated)
			assert.LessOrEqual(t, event.eventBytes, maxEventPayloadBytes)
		})
	}

	t.Run("Valid multi-byte UTF-8 is cut on rune boundary", func(t *testing.T) {
		runeBytes := len("日")
		// Pad with ASCII so that cutting at the byte budget would split a rune after its first byte.
		input := strings.Repeat("a", (budget-1)%runeBytes) + strings.Repeat("日", maxMessageSize/runeBytes+1)
		require.Greater(t, len(input), maxMessageSize)
		require.False(t, utf8.ValidString(input[:budget]), "cutting at the byte budget would split a rune")

		result, size, truncated := validateAndTruncateMessage(input)
		assert.True(t, truncated)
		require.True(t, strings.HasSuffix(result, defaultTruncationSuffix))
		kept := strings.TrimSuffix(result, defaultTruncationSuffix)
		assert.True(t, utf8.ValidString(kept))
		assert.Equal(t, input[:budget-1], kept)
		assert.Equal(t, len(result), size)
		assert.LessOrEqual(t, utf8EncodedLength(result), maxMessageSize)
	})
}

// TestValidateAndTruncateMessage_LengthBoundary covers raw lengths around maxMessageSize/invalidUTF8ByteSize, below
// which no message can be counted as more than maxMessageSize bytes. Measuring the message alone decides whether it
// fits, so these messages need no length-based shortcut.
func TestValidateAndTruncateMessage_LengthBoundary(t *testing.T) {
	maxMessageSize := maxEventPayloadBytes - perEventHeaderBytes
	n := maxMessageSize / invalidUTF8ByteSize
	require.Zero(t, n%6, "n must be a whole number of 2-byte and 3-byte runes")

	tests := []struct {
		name          string
		input         string
		expectedSize  int
		wantTruncated bool
	}{
		{
			name:         "Invalid UTF-8 at boundary",
			input:        string(bytes.Repeat([]byte{0xff}, n)),
			expectedSize: invalidUTF8ByteSize * n,
		},
		{
			name:          "Invalid UTF-8 one byte over boundary",
			input:         string(bytes.Repeat([]byte{0xff}, n+1)),
			wantTruncated: true,
		},
		{
			name:         "ASCII at boundary",
			input:        strings.Repeat("a", n),
			expectedSize: n,
		},
		{
			name:         "ASCII one byte over boundary",
			input:        strings.Repeat("a", n+1),
			expectedSize: n + 1,
		},
		{
			name:         "2-byte UTF-8 at boundary",
			input:        strings.Repeat("é", n/2),
			expectedSize: n,
		},
		{
			name:         "2-byte UTF-8 one byte over boundary",
			input:        strings.Repeat("é", n/2) + "a",
			expectedSize: n + 1,
		},
		{
			name:         "3-byte UTF-8 at boundary",
			input:        strings.Repeat("日", n/3),
			expectedSize: n,
		},
		{
			name:         "3-byte UTF-8 one byte over boundary",
			input:        strings.Repeat("日", n/3) + "a",
			expectedSize: n + 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, size, truncated := validateAndTruncateMessage(tt.input)
			assert.Equal(t, tt.wantTruncated, truncated)
			if tt.wantTruncated {
				assert.True(t, strings.HasSuffix(result, defaultTruncationSuffix))
				assert.NotEqual(t, tt.input, result)
				assert.LessOrEqual(t, size, maxMessageSize)
				return
			}
			assert.Equal(t, tt.input, result)
			assert.Equal(t, tt.expectedSize, size)
		})
	}
}

// TestTruncateToUTF8EncodedLength checks the prefix and size returned for every limit up to the counted size of each
// message, including limits of 0 or less, which keep nothing.
func TestTruncateToUTF8EncodedLength(t *testing.T) {
	for _, tc := range utf8EncodedLengthTestCases {
		t.Run(tc.name, func(t *testing.T) {
			for limit := -1; limit <= tc.expected; limit++ {
				prefix, size := truncateToUTF8EncodedLength(tc.message, limit)
				require.True(t, strings.HasPrefix(tc.message, prefix), "limit %d", limit)
				assert.Equal(t, utf8EncodedLength(prefix), size, "limit %d", limit)
				if limit <= 0 {
					assert.Empty(t, prefix, "limit %d", limit)
					continue
				}
				assert.LessOrEqual(t, size, limit, "limit %d", limit)
				if prefix == tc.message {
					assert.Equal(t, tc.expected, size)
					continue
				}
				// The prefix is the longest that fits: adding the next rune would exceed the limit.
				_, next := decodeRuneSize(tc.message[len(prefix):])
				assert.Greater(t, size+next, limit, "limit %d", limit)
			}
		})
	}
}
