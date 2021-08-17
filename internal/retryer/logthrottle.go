package retryer

import (
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

	throttleChan chan error
	done         chan struct{}

	client.DefaultRetryer
}

func NewLogThrottleRetryer(logger telegraf.Logger) *LogThrottleRetryer {
	r := &LogThrottleRetryer{
		Log:            logger,
		throttleChan:   make(chan error, 1),
		done:           make(chan struct{}),
		DefaultRetryer: client.DefaultRetryer{NumMaxRetries: client.DefaultRetryerMaxNumRetries},
	}

	go r.watchThrottleEvents()
	return r
}

func (r *LogThrottleRetryer) ShouldRetry(req *request.Request) bool {
	if req.IsErrorThrottle() {
		r.throttleChan <- req.Error
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

	var start time.Time
	var err error
	cnt := 0
	for {
		select {
		case err = <-r.throttleChan:
			// Log first throttle if there has not been any recent throttling events
			if cnt == 0 {
				if time.Since(start) > 2*throttleReportTimeout {
					r.Log.Infof("aws api call throttling detected: %v", err)
				} else {
					r.Log.Debugf("aws api call throttling detected: %v", err)
				}
				start = time.Now()
			} else {
				r.Log.Debugf("aws api call throttling detected: %v", err)
			}
			cnt++
		case <-ticker.C:
			if cnt == 0 {
				continue
			}
			d := time.Since(start)
			if d > throttleReportTimeout {
				if cnt > 1 {
					r.Log.Infof("aws api call has been throttled for %v times in the past %v, last throttle error message: %v", cnt, d, err)
				}
				cnt = 0
			}
		case <-r.done:
			if cnt > 0 {
				r.Log.Infof("aws api call has been throttled for %v times in the past %v, last throttle error message: %v", cnt, time.Since(start), err)
			}
			r.Log.Debugf("LogThrottleRetryer watch throttle events goroutine exiting")
			return
		}
	}
}
