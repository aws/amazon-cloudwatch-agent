// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

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

		event := newLogEvent(time.Now(), "Test message", nil)
		maxEvents := reqSizeLimit / event.eventBytes

		// Add events until close to the limit
		for i := 0; i < maxEvents-1; i++ {
			batch.append(event)
		}

		assert.True(t, batch.hasSpace(event.eventBytes))

		// Add one more event to reach the limit
		batch.append(event)

		assert.False(t, batch.hasSpace(event.eventBytes))
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
}
