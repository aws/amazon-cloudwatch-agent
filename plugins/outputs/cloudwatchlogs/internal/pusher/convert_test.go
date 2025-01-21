// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
)

type stubLogEvent struct {
	message   string
	timestamp time.Time
	done      func()
}

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

func TestConverter(t *testing.T) {
	logger := testutil.Logger{Name: "converter"}
	target := &Target{Group: "testGroup", Stream: "testStream"}

	t.Run("WithValidTimestamp", func(t *testing.T) {
		t.Parallel()
		now := time.Now()

		conv := newConverter(logger, target)
		le := conv.convert(newStubLogEvent("Test message", now))

		assert.Equal(t, now, le.timestamp)
		assert.Equal(t, "Test message", le.message)
		assert.Equal(t, now, conv.lastValidTime)
	})

	t.Run("WithNoTimestamp", func(t *testing.T) {
		t.Parallel()
		testTimestampMs := time.UnixMilli(12345678)

		conv := newConverter(logger, target)
		conv.lastValidTime = testTimestampMs

		le := conv.convert(newStubLogEvent("Test message", time.Time{}))

		assert.Equal(t, testTimestampMs, le.timestamp)
		assert.Equal(t, "Test message", le.message)
	})

	t.Run("TruncateMessage", func(t *testing.T) {
		t.Parallel()
		largeMessage := string(make([]byte, msgSizeLimit+100))
		event := newStubLogEvent(largeMessage, time.Now())

		conv := newConverter(logger, target)
		le := conv.convert(event)

		assert.Equal(t, msgSizeLimit, len(le.message))
		assert.Equal(t, truncatedSuffix, (le.message)[len(le.message)-len(truncatedSuffix):])
	})

	t.Run("WithOldTimestampWarning", func(t *testing.T) {
		oldTime := time.Now().Add(-25 * time.Hour)
		conv := newConverter(logger, target)
		conv.lastValidTime = oldTime
		conv.lastUpdateTime = oldTime

		var logbuf bytes.Buffer
		log.SetOutput(io.MultiWriter(&logbuf, os.Stdout))
		le := conv.convert(newStubLogEvent("Test message", time.Time{}))

		assert.Equal(t, oldTime, le.timestamp)
		assert.Equal(t, "Test message", le.message)
		loglines := strings.Split(strings.TrimSpace(logbuf.String()), "\n")
		assert.Len(t, loglines, 1)
		logline := loglines[0]
		assert.True(t, strings.Contains(logline, "W!"))
		assert.True(t, strings.Contains(logline, "Unable to parse timestamp"))
	})
}
