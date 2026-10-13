// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

type stubLogEvent struct {
	message   string
	timestamp time.Time
	done      func()
}

var _ logs.LogEvent = (*stubLogEvent)(nil)

func (m *stubLogEvent) Message() string {
	return m.message
}

func (m *stubLogEvent) Time() time.Time {
	return m.timestamp
}

func (m *stubLogEvent) Done() {
	if m.done != nil {
		m.done()
	}
}

func newStubLogEvent(message string, timestamp time.Time) *stubLogEvent {
	return &stubLogEvent{
		message:   message,
		timestamp: timestamp,
	}
}

type stubStatefulLogEvent struct {
	*stubLogEvent
	r     state.Range
	queue state.FileRangeQueue
}

var _ logs.StatefulLogEvent = (*stubStatefulLogEvent)(nil)

func (s *stubStatefulLogEvent) Range() state.Range {
	return s.r
}

func (s *stubStatefulLogEvent) RangeQueue() state.FileRangeQueue {
	return s.queue
}

func newStubStatefulLogEvent(
	message string,
	timestamp time.Time,
	r state.Range,
	queue state.FileRangeQueue,
) *stubStatefulLogEvent {
	return &stubStatefulLogEvent{
		stubLogEvent: newStubLogEvent(message, timestamp),
		r:            r,
		queue:        queue,
	}
}

type mockRangeQueue struct {
	mock.Mock
}

var _ state.FileRangeQueue = (*mockRangeQueue)(nil)

func (m *mockRangeQueue) ID() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockRangeQueue) Enqueue(state state.Range) {
	m.Called(state)
}

func TestConverter(t *testing.T) {
	logger := testutil.NewNopLogger()
	target := Target{Group: "testGroup", Stream: "testStream"}

	t.Run("WithValidTimestamp", func(t *testing.T) {
		t.Parallel()
		now := time.Now()

		conv := newConverter(logger, target)
		les := conv.convert(newStubLogEvent("Test message", now))
		require.Len(t, les, 1)
		le := les[0]

		assert.Equal(t, now, le.timestamp)
		assert.Equal(t, "Test message", le.message)
		assert.Equal(t, now, conv.lastValidTime)
		assert.Nil(t, le.state)
	})

	t.Run("WithNoTimestamp", func(t *testing.T) {
		t.Parallel()
		testTimestampMs := time.UnixMilli(12345678)

		conv := newConverter(logger, target)
		conv.lastValidTime = testTimestampMs

		les := conv.convert(newStubLogEvent("Test message", time.Time{}))
		require.Len(t, les, 1)
		le := les[0]

		assert.Equal(t, testTimestampMs, le.timestamp)
		assert.Equal(t, "Test message", le.message)
		assert.Nil(t, le.state)
	})

	t.Run("WithOldTimestampWarning", func(t *testing.T) {
		t.Parallel()
		oldTime := time.Now().Add(-25 * time.Hour)
		logSink := testutil.NewLogSink()
		conv := newConverter(logSink, target)
		conv.lastValidTime = oldTime
		conv.lastUpdateTime = oldTime

		les := conv.convert(newStubLogEvent("Test message", time.Time{}))
		require.Len(t, les, 1)
		le := les[0]

		assert.Equal(t, oldTime, le.timestamp)
		assert.Equal(t, "Test message", le.message)
		assert.Nil(t, le.state)
		logLines := logSink.Lines()
		assert.Len(t, logLines, 1)
		logLine := logLines[0]
		assert.True(t, strings.Contains(logLine, "W!"))
		assert.True(t, strings.Contains(logLine, "Unable to parse timestamp"))
	})

	t.Run("WithState", func(t *testing.T) {
		t.Parallel()
		now := time.Now()

		conv := newConverter(logger, target)
		r := state.NewRange(5, 10)
		mrq := &mockRangeQueue{}
		le := newStubStatefulLogEvent("Test message", now, r, mrq)
		gots := conv.convert(le)
		require.Len(t, gots, 1)
		got := gots[0]
		assert.NotNil(t, got.state)
		assert.NotNil(t, got.state.queue)
		assert.Equal(t, mrq, got.state.queue)
		assert.Equal(t, r, got.state.r)
	})

	// A message over the per-event limit is converted to several events with the source timestamp, and only the
	// last one carries the state and the done callback.
	t.Run("SplitWithState", func(t *testing.T) {
		t.Parallel()
		now := time.Now()

		conv := newConverter(logger, target)
		r := state.NewRange(5, 10)
		mrq := &mockRangeQueue{}
		message := strings.Repeat("a", 2*maxEventMessageBytes+1)
		le := newStubStatefulLogEvent(message, now, r, mrq)
		var doneCalls int
		le.done = func() { doneCalls++ }
		gots := conv.convert(le)
		require.Len(t, gots, 3)
		var reassembled strings.Builder
		for i, got := range gots {
			assert.Equal(t, now, got.timestamp, "event %d", i)
			assert.False(t, got.truncated, "event %d", i)
			reassembled.WriteString(got.message)
			if i < len(gots)-1 {
				assert.Nil(t, got.state, "event %d", i)
				assert.Nil(t, got.doneCallback, "event %d", i)
			}
		}
		assert.Equal(t, message, reassembled.String())
		last := gots[len(gots)-1]
		require.NotNil(t, last.state)
		assert.Equal(t, mrq, last.state.queue)
		assert.Equal(t, r, last.state.r)
		require.NotNil(t, last.doneCallback)
		last.doneCallback()
		assert.Equal(t, 1, doneCalls)
	})
}
