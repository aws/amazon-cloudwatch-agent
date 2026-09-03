// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

type mockFileRangeQueue struct {
	mock.Mock
}

func (m *mockFileRangeQueue) ID() string {
	return m.Called().String(0)
}

func (m *mockFileRangeQueue) Enqueue(r state.Range) {
	m.Called(r)
}

// newStatefulBatch creates a batch with stateful events that register state callbacks.
func newStatefulBatch(target Target, queue *mockFileRangeQueue) *logEventBatch {
	batch := newLogEventBatch(target, nil)
	now := time.Now()
	evt := newStatefulLogEvent(now, "test", nil, &logEventState{
		r:     state.NewRange(0, 100),
		queue: queue,
	})
	batch.append(evt)
	return batch
}

func okStubService(ple func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)) *stubLogsService {
	return &stubLogsService{
		ple: ple,
		cls: func(*cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		},
		clg: func(*cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
			return &cloudwatchlogs.CreateLogGroupOutput{}, nil
		},
		dlg: func(*cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
		},
	}
}

// readyBatch returns a batch already past its retry time so PopReady picks it up.
func readyBatch(group string, done, state func()) *logEventBatch {
	b := newLogEventBatch(Target{Group: group, Stream: "stream"}, nil)
	b.append(newLogEvent(time.Now(), "payload", func() {}))
	if done != nil {
		b.addDoneCallback(done)
	}
	if state != nil {
		b.addStateCallback(state)
	}
	b.nextRetryTime = time.Now().Add(-time.Second)
	return b
}

// stubSender is a no-op Sender for breaker-level tests.
type stubSender struct{}

func (*stubSender) Send(*logEventBatch) {}

func (*stubSender) Stop() {}

// panicSender panics inside Send to simulate a panic in the API call or a callback.
type panicSender struct{}

func (panicSender) Send(*logEventBatch) { panic("send boom") }

func (panicSender) Stop() {}
