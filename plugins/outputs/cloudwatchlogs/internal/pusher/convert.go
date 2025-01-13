// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"time"

	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/logs"
)

const (
	// Each log event can be no larger than 256 KB. When truncating the message, assume this is the limit for
	// message length.
	msgSizeLimit = 256*1024 - perEventHeaderBytes
	// The suffix to add to truncated log lines.
	truncatedSuffix = "[Truncated...]"

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

// Handles message truncation and timestamp
func (c *converter) convert(e logs.LogEvent) *logEvent {
	message := e.Message()

	if len(message) > msgSizeLimit {
		message = message[:msgSizeLimit-len(truncatedSuffix)] + truncatedSuffix
	}
	var t time.Time
	if e.Time().IsZero() {
		if !c.lastValidTime.IsZero() {
			// Where there has been a valid time before, assume most log events would have
			// a valid timestamp and use the last valid timestamp for new entries that does
			// not have a timestamp.
			t = c.lastValidTime
			if !c.lastUpdateTime.IsZero() {
				// Check when timestamp has an interval of 1 day.
				if time.Since(c.lastUpdateTime) > warnOldTimeStamp && time.Since(c.lastWarnMessage) > warnOldTimeStampLogInterval {
					c.logger.Warnf("Unable to parse timestamp, using last valid timestamp found in the logs %v: which is at least older than 1 day for log group %v: ", c.lastValidTime, c.Group)
					c.lastWarnMessage = time.Now()
				}
			}
		} else {
			t = time.Now()
		}
	} else {
		t = e.Time()
		c.lastValidTime = t
		c.lastUpdateTime = time.Now()
		c.lastWarnMessage = time.Time{}
	}
	return newLogEvent(t, message, e.Done)
}
