// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

type mockEntityProvider struct {
	mock.Mock
}

var _ logs.LogEntityProvider = (*mockEntityProvider)(nil)

func (m *mockEntityProvider) Entity() *cloudwatchlogs.Entity {
	args := m.Called()
	return args.Get(0).(*cloudwatchlogs.Entity)
}

func newMockEntityProvider(entity *cloudwatchlogs.Entity) *mockEntityProvider {
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
		testEntity := &cloudwatchlogs.Entity{
			Attributes: map[string]*string{
				"PlatformType":         aws.String("AWS::EC2"),
				"EC2.InstanceId":       aws.String("i-123456789"),
				"EC2.AutoScalingGroup": aws.String("test-group"),
			},
			KeyAttributes: map[string]*string{
				"Name":         aws.String("myService"),
				"Environment":  aws.String("myEnvironment"),
				"AwsAccountId": aws.String("123456789"),
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
}

func TestEventValidation_Over1MB(t *testing.T) {
	// Test event over 1MB - should be truncated with truncation suffix
	maxMessageSize := maxEventPayloadBytes - perEventHeaderBytes
	oversizeMessage := strings.Repeat("a", maxEventPayloadBytes+1000)

	event := newStatefulLogEvent(time.Now(), oversizeMessage, nil, nil)
	// The total length should still be maxMessageSize
	assert.Equal(t, maxMessageSize, len(event.message))
	assert.Equal(t, oversizeMessage[:maxMessageSize-len(truncationSuffix)]+truncationSuffix, event.message)
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
		name           string
		input          string
		expectedOutput string
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
			name:           "Over limit",
			input:          strings.Repeat("a", maxMessageSize+1000),
			expectedOutput: strings.Repeat("a", maxMessageSize-len(truncationSuffix)) + truncationSuffix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateAndTruncateMessage(tt.input)
			assert.Equal(t, tt.expectedOutput, result)
		})
	}
}
