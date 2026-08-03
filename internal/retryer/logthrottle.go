// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package retryer

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/influxdata/telegraf"
)

var (
	throttleReportTimeout     = 1 * time.Minute
	throttleReportCheckPeriod = 5 * time.Second

	// throttleChanBufferSize is the capacity of LogThrottleRetryer.throttleChan.
	// The original value of 1 dropped events when the watcher goroutine was
	// preempted under CI contention (6/200 lost). The non-blocking send in
	// ShouldRetry still guarantees the AWS SDK path never blocks at any size.
	throttleChanBufferSize = 1024
)

type LogThrottleRetryer struct {
	Log telegraf.Logger

	throttleChan chan throttleEvent
	done         chan struct{}
	stopped      chan struct{}

	client.DefaultRetryer
}

type throttleEvent struct {
	Operation string
	Err       error
}

func (te throttleEvent) String() string {
	return fmt.Sprintf("Operation: %v, Error: %v", te.Operation, te.Err)
}

func NewLogThrottleRetryer(logger telegraf.Logger) *LogThrottleRetryer {
	r := &LogThrottleRetryer{
		Log:            logger,
		throttleChan:   make(chan throttleEvent, throttleChanBufferSize),
		done:           make(chan struct{}),
		stopped:        make(chan struct{}),
		DefaultRetryer: client.DefaultRetryer{NumMaxRetries: client.DefaultRetryerMaxNumRetries},
	}

	go r.watchThrottleEvents()
	return r
}

func (r *LogThrottleRetryer) ShouldRetry(req *request.Request) bool {
	if req.IsErrorThrottle() {
		te := throttleEvent{Err: req.Error}
		if req.Operation != nil {
			te.Operation = req.Operation.Name
		}
		// Non-blocking: never block ShouldRetry if the consumer has stopped.
		select {
		case r.throttleChan <- te:
		default:
		}
	}

	// Fallback to SDK's built in retry rules
	return r.DefaultRetryer.ShouldRetry(req)
}

func (r *LogThrottleRetryer) Stop() {
	if r != nil {
		close(r.done)
		// Block until the watcher has exited and drained throttleChan, so callers
		// (notably tests counting aggregated throttles) don't race the final events.
		<-r.stopped
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
			// Drain queued events before returning: Go's select is randomized when
			// multiple cases are ready, so a naive return can strand events enqueued
			// between the last iteration and Stop().
		drainLoop:
			for {
				select {
				case event := <-r.throttleChan:
					process(event)
				default:
					break drainLoop
				}
			}
			if aggregatedCnt > 0 {
				r.Log.Infof("AWS API call has been throttled %v times in the past %v, last throttle error message: %v", aggregatedCnt, time.Since(lastReportTime), te)
			}
			r.Log.Debugf("LogThrottleRetryer watch throttle events goroutine exiting")
			return
		}
	}
}
