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
	// Setting this to 1 (the original value) means any pair of consecutive
	// ShouldRetry calls where the watcher goroutine has not yet been scheduled
	// causes the second event to be silently dropped by the non-blocking send
	// in ShouldRetry. Under Windows CI `make test` contention the watcher can
	// be preempted for tens of ms and drops accumulate quickly (observed 6
	// drops out of 200 in run 30110009561 baseline iter 8). Sizing the buffer
	// well above realistic bursts eliminates the drop path in practice; if a
	// caller ever exceeds this the non-blocking `default` still guarantees
	// ShouldRetry never blocks the AWS SDK error path.
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
		// Block until watchThrottleEvents has fully exited (including draining
		// any events still queued in throttleChan). Without this synchronization
		// callers -- most notably tests that count aggregated throttles -- can
		// race the watcher goroutine and observe fewer events than sent.
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
			// Drain any events still queued in throttleChan before returning.
			// The Go select is randomized when multiple cases are ready, so a
			// naïve return here can strand events that were enqueued between
			// the last iteration and Stop(). Draining eliminates the shutdown
			// race that surfaced as TestLogThrottleRetryerLogging reporting
			// "expecting 200, got N<200" under Windows CI contention
			// (run 30110009561 baseline iter 8).
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
