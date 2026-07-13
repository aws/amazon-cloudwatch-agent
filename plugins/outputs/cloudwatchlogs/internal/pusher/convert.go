// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"time"

	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/logs"
)

const (
	// The duration until a timestamp is considered old.
	warnOldTimeStamp = 24 * time.Hour
	// The minimum interval between logs warning about the old timestamps.
	warnOldTimeStampLogInterval = 5 * time.Minute
)

type converter struct {
	Target
	logger          telegraf.Logger
	lastValidTime   time.Time
	lastUpdateTime  time.Time
	lastWarnMessage time.Time
}

func newConverter(logger telegraf.Logger, target Target) *converter {
	return &converter{
		logger: logger,
		Target: target,
	}
}

// convert sets a timestamp if not set in the logs.LogEvent.
func (c *converter) convert(e logs.LogEvent) *logEvent {
	message := e.Message()

	now := time.Now()
	var t time.Time
	if e.Time().IsZero() {
		if !c.lastValidTime.IsZero() {
			// Where there has been a valid time before, assume most log events would have
			// a valid timestamp and use the last valid timestamp for new entries that does
			// not have a timestamp.
			t = c.lastValidTime
			if !c.lastUpdateTime.IsZero() {
				// Check when timestamp has an interval of 1 day.
				if now.Sub(c.lastUpdateTime) > warnOldTimeStamp && now.Sub(c.lastWarnMessage) > warnOldTimeStampLogInterval {
					c.logger.Warnf("Unable to parse timestamp, using last valid timestamp found in the logs %v: which is at least older than 1 day for log group %v: ", c.lastValidTime, c.Group)
					c.lastWarnMessage = now
				}
			}
		} else {
			t = now
		}
	} else {
		t = e.Time()
		c.lastValidTime = t
		c.lastUpdateTime = now
		c.lastWarnMessage = time.Time{}
	}
	var state *logEventState
	if sle, ok := e.(logs.StatefulLogEvent); ok {
		state = &logEventState{
			r:     sle.Range(),
			queue: sle.RangeQueue(),
		}
	}
	return newStatefulLogEvent(t, message, e.Done, state)
}
