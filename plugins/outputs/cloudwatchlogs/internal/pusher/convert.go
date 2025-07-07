// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"time"

	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/logs"
)

const (
	// Each log event can be no larger than 256 KB. Since the input plugin now handles
	// truncation with proper header accounting, we don't need to truncate again here.
	// This constant is kept for reference but truncation logic is removed.
	msgSizeLimit = 256*1024 - perEventHeaderBytes
	// The suffix to add to truncated log lines (handled by input plugin now).
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

// convert handles timestamp setting for logs.LogEvent. Message truncation is now handled
// by the input plugin to prevent double truncation and ensure consistent size limits.
func (c *converter) convert(e logs.LogEvent) *logEvent {
	message := e.Message()
	messageSize := len(message)

	// Detailed logging for output plugin processing
	c.logger.Debugf("[OUTPUT DEBUG] Processing log event:")
	c.logger.Debugf("  - Log Group: %s", c.Group)
	c.logger.Debugf("  - Log Stream: %s", c.Stream)
	c.logger.Debugf("  - Message size: %d bytes", messageSize)
	c.logger.Debugf("  - Message size limit (reference): %d bytes", msgSizeLimit)
	c.logger.Debugf("  - Per event header bytes: %d bytes", perEventHeaderBytes)
	
	// Check if message appears to be truncated (contains truncation suffix)
	if len(message) >= len(truncatedSuffix) && message[len(message)-len(truncatedSuffix):] == truncatedSuffix {
		c.logger.Infof("[OUTPUT DEBUG] Truncated message detected:")
		c.logger.Infof("  - Log Group: %s", c.Group)
		c.logger.Infof("  - Message size: %d bytes", messageSize)
		c.logger.Infof("  - Contains truncation suffix: %s", truncatedSuffix)
		c.logger.Infof("  - Message preview (first 200 chars): %.200s", message)
		c.logger.Infof("  - Message preview (last 200 chars): %s", message[max(0, len(message)-200):])
	}
	
	// Log if message is close to size limits
	if messageSize > msgSizeLimit*4/5 { // 80% of limit
		c.logger.Warnf("[OUTPUT DEBUG] Large message detected (>80%% of limit):")
		c.logger.Warnf("  - Log Group: %s", c.Group)
		c.logger.Warnf("  - Message size: %d bytes (%.1f%% of limit)", messageSize, float64(messageSize)/float64(msgSizeLimit)*100)
		c.logger.Warnf("  - Size limit: %d bytes", msgSizeLimit)
	}

	// Remove truncation logic here since it's now handled by the input plugin
	// This prevents double truncation and ensures consistent size handling
	
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
