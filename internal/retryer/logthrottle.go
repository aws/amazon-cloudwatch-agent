// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package retryer

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/smithy-go"
	"github.com/influxdata/telegraf"
)

const (
	// DefaultMaxRetries is the number of retries after the first attempt.
	DefaultMaxRetries = 3
)

var (
	throttleReportTimeout     = 1 * time.Minute
	throttleReportCheckPeriod = 5 * time.Second

	// stopTimeout bounds how long Stop() waits for the watcher to finish draining,
	// so a slow logger can't stall a caller (cloudwatchlogs holds a lock across Stop).
	stopTimeout = 5 * time.Second

	// throttleChanBufferSize is the capacity of LogThrottleRetryer.throttleChan.
	// The original value of 1 dropped events when the watcher goroutine was
	// preempted under load (observed ~6/200 lost under CI contention). 128 holds
	// a full burst with headroom while bounding memory (~32 bytes/slot). The
	// non-blocking send in IsErrorRetryable still guarantees the AWS SDK path never
	// blocks regardless of this value.
	throttleChanBufferSize = 128
)

type LogThrottleRetryer struct {
	Log telegraf.Logger

	throttleChan chan throttleEvent
	done         chan struct{}
	stopped      chan struct{}
	stopOnce     sync.Once

	// Embed the standard retryer for default behavior
	*retry.Standard
}

var _ aws.RetryerV2 = (*LogThrottleRetryer)(nil)

type throttleEvent struct {
	Operation string
	Err       error
}

func (te throttleEvent) String() string {
	return fmt.Sprintf("Operation: %v, Error: %v", te.Operation, te.Err)
}

func NewLogThrottleRetryer(logger telegraf.Logger) *LogThrottleRetryer {
	r := &LogThrottleRetryer{
		Log:          logger,
		throttleChan: make(chan throttleEvent, throttleChanBufferSize),
		done:         make(chan struct{}),
		stopped:      make(chan struct{}),
		Standard: retry.NewStandard(func(o *retry.StandardOptions) {
			o.MaxAttempts = DefaultMaxRetries + 1 // MaxAttempts includes the first attempt
		}),
	}

	go r.watchThrottleEvents()
	return r
}

func (r *LogThrottleRetryer) IsErrorRetryable(err error) bool {
	if IsErrThrottle(err) {
		te := throttleEvent{Err: err}
		var oe *smithy.OperationError
		if errors.As(err, &oe) {
			te.Operation = oe.OperationName
		}
		// Non-blocking: never block IsErrorRetryable if the consumer has stopped.
		select {
		case r.throttleChan <- te:
		default:
		}
	}

	// Fallback to SDK's built in retry rules
	return r.Standard.IsErrorRetryable(err)
}

func (r *LogThrottleRetryer) Stop() {
	if r != nil {
		// sync.Once guards against a double Stop() panicking on close(r.done).
		r.stopOnce.Do(func() {
			close(r.done)
			// Block until the watcher has exited after draining, so callers (notably
			// tests counting aggregated throttles) don't race the final events. Bounded
			// so a slow/blocking logger can't stall shutdown (one caller holds a lock here).
			select {
			case <-r.stopped:
			case <-time.After(stopTimeout):
			}
		})
	}
}

func (r *LogThrottleRetryer) watchThrottleEvents() {
	// Always signal completion so Stop() can return synchronously.
	defer close(r.stopped)
	ticker := time.NewTicker(throttleReportCheckPeriod)
	defer ticker.Stop()

	var lastReportTime time.Time
	var te throttleEvent
	aggregatedCnt := 0

	// process is defined as a closure so both the main loop and the drain-on-
	// shutdown block can use identical accounting logic.
	process := func(event throttleEvent) {
		te = event
		if time.Since(lastReportTime) >= throttleReportTimeout {
			r.Log.Infof("AWS API call throttling detected, further throttling messages may be suppressed for up to %v depending on the log level, error message: %v", throttleReportTimeout, te)
			lastReportTime = time.Now()
		} else {
			r.Log.Debugf("AWS API call throttled: %v", te)
		}
		aggregatedCnt++
	}

	for {
		select {
		case event := <-r.throttleChan:
			process(event)
		case <-ticker.C:
			d := time.Since(lastReportTime)
			if d > throttleReportTimeout {
				if aggregatedCnt > 0 {
					r.Log.Infof("AWS API call has been throttled %v times in the past %v, last throttle error message: %v", aggregatedCnt, d, te)
					aggregatedCnt = 0
				}
				lastReportTime = time.Now()
			}
		case <-r.done:
			// Drain the events already queued, then return. Go's select is randomized
			// when multiple cases are ready, so a naive return can strand events enqueued
			// between the last iteration and Stop(). Bound the drain to a snapshot of the
			// current length so it terminates even if a late IsErrorRetryable enqueues
			// more (those simply drop, as they would after the watcher exits).
			for n := len(r.throttleChan); n > 0; n-- {
				process(<-r.throttleChan)
			}
			if aggregatedCnt > 0 {
				r.Log.Infof("AWS API call has been throttled %v times in the past %v, last throttle error message: %v", aggregatedCnt, time.Since(lastReportTime), te)
			}
			r.Log.Debugf("LogThrottleRetryer watch throttle events goroutine exiting")
			return
		}
	}
}

// IsErrThrottle is a wrapper for the default throttle error code check for the AWS SDK retry logic.
func IsErrThrottle(err error) bool {
	return retry.IsErrorThrottles(retry.DefaultThrottles).IsErrorThrottle(err) == aws.TrueTernary
}
