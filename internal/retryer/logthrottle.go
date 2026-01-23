// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package retryer

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/smithy-go"
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
		throttleChan: make(chan throttleEvent, 1),
		done:         make(chan struct{}),
		Standard:     retry.NewStandard(),
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
		r.throttleChan <- te
	}

	// Fallback to SDK's built in retry rules
	return r.Standard.IsErrorRetryable(err)
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

// IsErrThrottle is a wrapper for the default throttle error code check for the AWS SDK retry logic.
func IsErrThrottle(err error) bool {
	return retry.IsErrorThrottles(retry.DefaultThrottles).IsErrorThrottle(err) == aws.TrueTernary
}
