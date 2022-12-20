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
)

type LogThrottleRetryer struct {
	Log telegraf.Logger

	throttleChan chan throttleEvent
	done         chan struct{}

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
		throttleChan:   make(chan throttleEvent, 1),
		done:           make(chan struct{}),
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
		r.throttleChan <- te
	}

	// Fallback to SDK's built in retry rules
	return r.DefaultRetryer.ShouldRetry(req)
}

func (r *LogThrottleRetryer) Stop() {
	if r != nil {
		close(r.done)
	}
}

func (r *LogThrottleRetryer) watchThrottleEvents() {
	ticker := time.NewTicker(throttleReportCheckPeriod)
	defer ticker.Stop()

	var lastReportTime time.Time
	var te throttleEvent
	aggregatedCnt := 0
	for {
		select {
		case te = <-r.throttleChan:
			if time.Since(lastReportTime) >= throttleReportTimeout {
				r.Log.Infof("AWS API call throttling detected, further throttling messages may be suppressed for up to %v depending on the log level, error message: %v", throttleReportTimeout, te)
				lastReportTime = time.Now()
			} else {
				r.Log.Debugf("AWS API call throttled: %v", te)
			}
			aggregatedCnt++
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
			if aggregatedCnt > 0 {
				r.Log.Infof("AWS API call has been throttled %v times in the past %v, last throttle error message: %v", aggregatedCnt, time.Since(lastReportTime), te)
			}
			r.Log.Debugf("LogThrottleRetryer watch throttle events goroutine exiting")
			return
		}
	}
}
