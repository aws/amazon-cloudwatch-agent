// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"context"
	"math/rand"
	"sort"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/profiler"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/influxdata/telegraf"
)

const (
	reqSizeLimit   = 1024 * 1024
	reqEventsLimit = 10000
)

var (
	seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type CloudWatchLogsService interface {
	PutLogEvents(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)
	CreateLogStream(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error)
	CreateLogGroup(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error)
}

type pusher struct {
	Target
	Service       CloudWatchLogsService
	FlushTimeout  time.Duration
	RetryDuration time.Duration
	Log           telegraf.Logger

	events        []*cloudwatchlogs.InputLogEvent
	minT, maxT    *time.Time
	doneCallbacks []func()
	eventsCh      chan logs.LogEvent
	bufferredSize int
	flushTimer    *time.Timer
	sequenceToken *string
	lastValidTime int64
	needSort      bool
	ctx           context.Context
	cancelFn      func()
}

func NewPusher(target Target, service CloudWatchLogsService, flushTimeout time.Duration, retryDuration time.Duration, logger telegraf.Logger) *pusher {
	ctx, cancel := context.WithCancel(context.Background())
	cwl, ok := service.(*cloudwatchlogs.CloudWatchLogs)
	if ok {
		cwl.Handlers.Build.PushBack(func(req *request.Request) {
			req.SetContext(ctx)
		})
	}
	p := &pusher{
		Target:        target,
		Service:       service,
		FlushTimeout:  flushTimeout,
		RetryDuration: retryDuration,
		Log:           logger,

		events:     make([]*cloudwatchlogs.InputLogEvent, 0, 10),
		eventsCh:   make(chan logs.LogEvent, 100),
		flushTimer: time.NewTimer(flushTimeout),
		ctx:        ctx,
		cancelFn:   cancel,
	}
	go p.start()
	return p
}

func (p *pusher) AddEvent(e logs.LogEvent) {
	if !hasValidTime(e) {
		p.Log.Errorf("The log entry in (%v/%v) with timestamp (%v) comparing to the current time (%v) is out of accepted time range. Discard the log entry.", p.Group, p.Stream, e.Time(), time.Now())
		return
	}
	p.eventsCh <- e
}

func (p *pusher) AddEventNonBlocking(e logs.LogEvent) {
	if !hasValidTime(e) {
		p.Log.Errorf("The log entry in (%v/%v) with timestamp (%v) comparing to the current time (%v) is out of accepted time range. Discard the log entry.", p.Group, p.Stream, e.Time(), time.Now())
		return
	}
	// Drain the channel until new event can be added
	for {
		select {
		case p.eventsCh <- e:
			return
		default:
			<-p.eventsCh
		}
	}
}

func hasValidTime(e logs.LogEvent) bool {
	//http://docs.aws.amazon.com/goto/SdkForGoV1/logs-2014-03-28/PutLogEvents
	//* None of the log events in the batch can be more than 2 hours in the future.
	//* None of the log events in the batch can be older than 14 days or the retention period of the log group.
	if !e.Time().IsZero() {
		now := time.Now()
		dt := now.Sub(e.Time()).Hours()
		if dt > 24*14 || dt < -2 {
			return false
		}
	}
	return true
}

func (p *pusher) Stop() {
	p.cancelFn()
}

func (p *pusher) start() {
	for {
		select {
		case e := <-p.eventsCh:
			if len(p.events) == 0 {
				p.resetFlushTimer()
			}

			// A batch of log events in a single request cannot span more than 24 hours.
			et := e.Time()
			if !et.IsZero() && // event with zero time is handled by convertEvent method
				((p.minT != nil && et.Sub(*p.minT) > 24*time.Hour) || (p.maxT != nil && p.maxT.Sub(et) > 24*time.Hour)) {
				p.send()
			}

			ce := p.convertEvent(e)
			size := len(*ce.Message) + eventHeaderSize
			if p.bufferredSize+size > reqSizeLimit || len(p.events) == reqEventsLimit {
				p.send()
			}

			if len(p.events) > 0 && *ce.Timestamp < *p.events[len(p.events)-1].Timestamp {
				p.needSort = true
			}

			p.events = append(p.events, ce)
			p.doneCallbacks = append(p.doneCallbacks, e.Done)
			p.bufferredSize += size
			if p.minT == nil || (!et.IsZero() && p.minT.After(et)) {
				p.minT = &et
			}
			if p.maxT == nil || p.maxT.Before(et) {
				p.maxT = &et
			}

		case <-p.flushTimer.C:
			if len(p.events) > 0 {
				p.send()
			}
		case <-p.ctx.Done():
			if len(p.events) > 0 {
				p.send()
			}
			return
		}
	}
}

func (p *pusher) reset() {
	p.events = p.events[:0]
	p.doneCallbacks = p.doneCallbacks[:0]
	p.bufferredSize = 0
	p.needSort = false
	p.minT = nil
	p.maxT = nil
}

func (p *pusher) send() {
	if p.needSort {
		sort.Stable(ByTimestamp(p.events))
	}

	input := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     p.events,
		LogGroupName:  &p.Group,
		LogStreamName: &p.Stream,
		SequenceToken: p.sequenceToken,
	}

	startTime := time.Now()

	retryCount := 0
	for {
		input.SequenceToken = p.sequenceToken
		output, err := p.Service.PutLogEvents(input)
		if err == nil {
			if output.NextSequenceToken != nil {
				p.sequenceToken = output.NextSequenceToken
			}
			if output.RejectedLogEventsInfo != nil {
				info := output.RejectedLogEventsInfo
				if info.TooOldLogEventEndIndex != nil {
					p.Log.Warnf("%d log events for log '%s/%s' are too old", *info.TooOldLogEventEndIndex, p.Group, p.Stream)
				}
				if info.TooNewLogEventStartIndex != nil {
					p.Log.Warnf("%d log events for log '%s/%s' are too new", *info.TooNewLogEventStartIndex, p.Group, p.Stream)
				}
				if info.ExpiredLogEventEndIndex != nil {
					p.Log.Warnf("%d log events for log '%s/%s' are expired", *info.ExpiredLogEventEndIndex, p.Group, p.Stream)
				}
			}

			for _, done := range p.doneCallbacks {
				done()
			}

			p.Log.Debugf("Pusher published %v log events to group: %v stream: %v with size %v KB in %v.", len(p.events), p.Group, p.Stream, p.bufferredSize/1024, time.Since(startTime))
			p.addStats("rawSize", float64(p.bufferredSize))

			p.reset()

			return
		}

		awsErr, ok := err.(awserr.Error)
		if !ok {
			p.Log.Errorf("Non aws error received when sending logs to %v/%v: %v", p.Group, p.Stream, err)
			// Messages will be discarded but done callbacks not called
			p.reset()
			return
		}

		switch e := awsErr.(type) {
		case *cloudwatchlogs.ResourceNotFoundException:
			err := p.createLogGroupAndStream()
			if err != nil {
				p.Log.Errorf("Unable to create log stream %v/%v: %v", p.Group, p.Stream, e.Message())
			}
		case *cloudwatchlogs.InvalidSequenceTokenException:
			p.Log.Warnf("Invalid SequenceToken used, will use new token and retry: %v", e.Message())
			if e.ExpectedSequenceToken == nil {
				p.Log.Errorf("Failed to find sequence token from aws response while sending logs to %v/%v: %v", p.Group, p.Stream, e.Message())
			}
			p.sequenceToken = e.ExpectedSequenceToken
		case *cloudwatchlogs.InvalidParameterException,
			*cloudwatchlogs.DataAlreadyAcceptedException:
			p.Log.Errorf("%v, will not retry the request", e)
			p.reset()
			return
		default:
			p.Log.Errorf("Aws error received when sending logs to %v/%v: %v", p.Group, p.Stream, awsErr)
		}

		wait := retryWait(retryCount)
		if time.Since(startTime)+wait > p.RetryDuration {
			p.Log.Errorf("All %v retries to %v/%v failed for PutLogEvents, request dropped.", p.Group, p.Stream, retryCount)
		}

		p.Log.Warnf("Retried %v time, going to sleep %v before retrying.", retryCount, wait)
		time.Sleep(wait)
		retryCount++
	}

}

func retryWait(n int) time.Duration {
	const base = 200 * time.Millisecond
	const max = 1 * time.Minute
	d := base * time.Duration(1<<int64(n))
	if n > 5 {
		d = max
	}
	return time.Duration(seededRand.Int63n(int64(d/2)) + int64(d/2))
}

func (p *pusher) createLogGroupAndStream() error {
	_, err := p.Service.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: &p.Group,
	})
	if err != nil {
		awsErr, ok := err.(awserr.Error)
		if !ok || awsErr.Code() != cloudwatchlogs.ErrCodeResourceAlreadyExistsException {
			return err
		}
	}

	_, err = p.Service.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  &p.Group,
		LogStreamName: &p.Stream,
	})

	return err
}

func (p *pusher) resetFlushTimer() {
	p.flushTimer.Stop()
	p.flushTimer.Reset(p.FlushTimeout)
}

func (p *pusher) convertEvent(e logs.LogEvent) *cloudwatchlogs.InputLogEvent {
	message := e.Message()

	if len(message) > msgSizeLimit {
		message = message[:msgSizeLimit-len(truncatedSuffix)] + truncatedSuffix
	}
	var t int64
	if e.Time().IsZero() {
		if p.lastValidTime != 0 {
			// Where there has been a valid time before, assume most log events would have
			// a valid timestamp and use the last valid timestamp for new entries that does
			// not have a timestamp.
			t = p.lastValidTime
		} else {
			t = time.Now().UnixNano() / 1000000
		}
	} else {
		t = e.Time().UnixNano() / 1000000
		p.lastValidTime = t
	}
	return &cloudwatchlogs.InputLogEvent{
		Message:   &message,
		Timestamp: &t,
	}
}

func (p *pusher) addStats(statsName string, value float64) {
	statsKey := []string{"cloudwatchlogs", p.Group, statsName}
	profiler.Profiler.AddStats(statsKey, value)
}

type ByTimestamp []*cloudwatchlogs.InputLogEvent

func (inputLogEvents ByTimestamp) Len() int {
	return len(inputLogEvents)
}

func (inputLogEvents ByTimestamp) Swap(i, j int) {
	inputLogEvents[i], inputLogEvents[j] = inputLogEvents[j], inputLogEvents[i]
}

func (inputLogEvents ByTimestamp) Less(i, j int) bool {
	return *inputLogEvents[i].Timestamp < *inputLogEvents[j].Timestamp
}
