// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/google/uuid"
	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/handlers/agentinfo"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/profiler"
	"github.com/aws/amazon-cloudwatch-agent/tool/persistentqueue"
	"github.com/aws/amazon-cloudwatch-agent/tool/persistentqueue/diskqueue"
)

const (
	reqSizeLimit         = 1024 * 1024
	reqEventsLimit       = 10000
	queuesDirectory      = "/var/agent/queues"
	maxBacklogQueueDepth = 1024 // 100 MB file with 100 KB per message in worst case (1024 = 100 MB / 100 KB)

	maxBytesPerFile = 1024 * 1024 * 1024 // 100 MB
	maxMsgSize      = 1024 * 1024        // 100 KB
	minMsgSize      = 1                  // messages should have content
	syncEvery       = 5
	syncTimeout     = time.Minute
)

var (
	seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type CloudWatchLogsService interface {
	PutLogEvents(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)
	CreateLogStream(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error)
	CreateLogGroup(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error)
	PutRetentionPolicy(input *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
}

type pusher struct {
	Target
	Service       CloudWatchLogsService
	FlushTimeout  time.Duration
	RetryDuration time.Duration
	Log           telegraf.Logger

	events              []*cloudwatchlogs.InputLogEvent
	minT, maxT          *time.Time
	doneCallbacks       []func()
	eventsCh            chan logs.LogEvent
	nonBlockingEventsCh chan logs.LogEvent
	bufferredSize       int
	flushTimer          *time.Timer
	sequenceToken       *string
	lastValidTime       int64
	needSort            bool
	stop                <-chan struct{}
	lastSentTime        time.Time

	backlogQueue      persistentqueue.PersistentQueue
	ticker            *time.Ticker
	dequeueEvents     *cloudwatchlogs.PutLogEventsInput
	backlogBufferSize int

	initNonBlockingChOnce sync.Once
	startNonBlockCh       chan struct{}
	wg                    *sync.WaitGroup
	agentInfo             agentinfo.AgentInfo
}

func NewPusher(target Target, service CloudWatchLogsService, flushTimeout time.Duration, retryDuration time.Duration, logger telegraf.Logger, stop <-chan struct{}, wg *sync.WaitGroup, agentInfo agentinfo.AgentInfo) *pusher {
	marshaler := func(i interface{}) ([]byte, error) { return json.Marshal(i) }
	unmarshaler := func(bytes []byte) (interface{}, error) {
		var data retryStruct
		return data, json.Unmarshal(bytes, &data)
	}
	id := uuid.New()
	dn := fmt.Sprintf("dir_%s", id.String())
	p := &pusher{
		Target:        target,
		Service:       service,
		FlushTimeout:  flushTimeout,
		RetryDuration: retryDuration,
		Log:           logger,
		events:        make([]*cloudwatchlogs.InputLogEvent, 0, 10),
		eventsCh:      make(chan logs.LogEvent, 100),
		flushTimer:    time.NewTimer(flushTimeout),
		stop:          stop,
		backlogQueue: diskqueue.NewPersistentQueue(
			dn,
			queuesDirectory,
			maxBacklogQueueDepth,
			maxBytesPerFile,
			minMsgSize,
			maxMsgSize,
			syncEvery,
			syncTimeout,
			marshaler,
			unmarshaler,
			logger,
		),
		startNonBlockCh: make(chan struct{}),
		wg:              wg,
		agentInfo:       agentInfo,
	}
	p.putRetentionPolicy()
	p.wg.Add(1)
	p.Log.Debugf("Pusher has been created,run start()")
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

	p.initNonBlockingChOnce.Do(func() {
		p.nonBlockingEventsCh = make(chan logs.LogEvent, reqEventsLimit*2)
		p.startNonBlockCh <- struct{}{} // Unblock the select loop to recogonize the channel merge
	})

	// Drain the channel until new event can be added
	for {
		select {
		case p.nonBlockingEventsCh <- e:
			return
		default:
			<-p.nonBlockingEventsCh
			p.addStats("emfMetricDrop", 1)
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

func (p *pusher) start() {
	defer p.wg.Done()
	//var tmpwg sync.WaitGroup
	//tmpwg.Add(1)
	ec := make(chan logs.LogEvent)
	p.Log.Debugf("ticker been created")
	p.ticker = time.NewTicker(5 * time.Second)

	// Merge events from both blocking and non-blocking channel
	go func() {
		for {
			select {
			case e := <-p.eventsCh:
				ec <- e
			case e := <-p.nonBlockingEventsCh:
				ec <- e
			case <-p.startNonBlockCh:
			case <-p.stop:
				return
			}
		}
	}()

	for {
		select {
		case <-p.ticker.C:
			p.Log.Debugf("start dequeue from disk")
			temp, bufferSize, err := p.dequeue()
			if err != nil {
				p.Log.Errorf("Error while dequeue log from the backlogQueue")

			} else {
				if temp != nil {
					p.dequeueEvents = temp
					p.backlogBufferSize = bufferSize
					p.Log.Debugf("send dequeue logEvent to cloudwatch")
					p.send()
				} else {
					p.Log.Debugf("there is no logEvent in disk")
				}
			}
		case e := <-ec:
			// Start timer when first event of the batch is added (happens after a flush timer timeout)
			if len(p.events) == 0 {
				p.resetFlushTimer()
			}

			ce := p.convertEvent(e)
			et := time.Unix(*ce.Timestamp/1000, *ce.Timestamp%1000) // Cloudwatch Log Timestamp is in Millisecond

			// A batch of log events in a single request cannot span more than 24 hours.
			if (p.minT != nil && et.Sub(*p.minT) > 24*time.Hour) || (p.maxT != nil && p.maxT.Sub(et) > 24*time.Hour) {
				p.send()
			}

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
			if p.minT == nil || p.minT.After(et) {
				p.minT = &et
			}
			if p.maxT == nil || p.maxT.Before(et) {
				p.maxT = &et
			}
		case <-p.flushTimer.C:
			if time.Since(p.lastSentTime) >= p.FlushTimeout && len(p.events) > 0 {
				p.send()
			} else {
				p.resetFlushTimer()
			}
		case <-p.stop:
			if len(p.events) > 0 {
				p.send()
			}
			return
		}
	}
}

func (p *pusher) dequeue() (*cloudwatchlogs.PutLogEventsInput, int, error) {
	if p.backlogQueue.Depth() == 0 {
		return nil, 0, nil
	}
	obj, err := p.backlogQueue.Dequeue()
	if err != nil {
		p.Log.Debugf("errors happens when dequeue from disk")
		return nil, 0, err
	}
	input := obj.(retryStruct)
	if !input.FirstRetryTime.IsZero() {
		now := time.Now()
		dt := now.Sub(input.FirstRetryTime).Hours()
		if dt > 24*14 || dt < -2 {
			p.Log.Errorf("The log entry in (%v/%v) with timestamp (%v) comparing to the current time (%v) is out of accepted time range. Discard the log entry.", p.Group, p.Stream, input.FirstRetryTime, time.Now())
			return nil, 0, nil
		}
	}

	return input.PutLogEventsInput, input.BufferredSize, nil
}

func (p *pusher) reset() {
	for i := 0; i < len(p.events); i++ {
		p.events[i] = nil
	}
	p.events = p.events[:0]
	for i := 0; i < len(p.doneCallbacks); i++ {
		p.doneCallbacks[i] = nil
	}
	p.doneCallbacks = p.doneCallbacks[:0]
	p.bufferredSize = 0
	p.needSort = false
	p.minT = nil
	p.maxT = nil
}

type retryStruct struct {
	*cloudwatchlogs.PutLogEventsInput
	FirstRetryTime time.Time `type:"firstRetryTimeStamp"`
	BufferredSize  int       `type:"buffersize"`
}

func (p *pusher) send() {
	var input *cloudwatchlogs.PutLogEventsInput
	startTime := time.Now()
	defer p.resetFlushTimer() // Reset the flush timer after sending the request

	if p.dequeueEvents != nil {
		p.Log.Debug("send dequeueEvents to cloudwatch")
		input = p.dequeueEvents
		p.dequeueEvents = nil
		input.SequenceToken = p.sequenceToken
		opStartTime := time.Now()
		output, err := p.Service.PutLogEvents(input)
		p.agentInfo.RecordOpData(time.Since(opStartTime), p.backlogBufferSize, err)
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
		}
		p.Log.Debugf("in the persistentQueue Pusher published %v log events to group: %v stream: %v with size %v KB in %v.", len(input.LogEvents), *input.LogGroupName, *input.LogStreamName, p.backlogBufferSize/1024, time.Since(startTime))
		p.addStats("rawSize", float64(p.backlogBufferSize))
		p.backlogBufferSize = 0
		return
	}

	if p.needSort {
		sort.Stable(ByTimestamp(p.events))
	}

	input = &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     p.events,
		LogGroupName:  &p.Group,
		LogStreamName: &p.Stream,
		SequenceToken: p.sequenceToken,
	}

	retryCount := 0
	for {
		input.SequenceToken = p.sequenceToken
		opStartTime := time.Now()
		output, err := p.Service.PutLogEvents(input)
		p.agentInfo.RecordOpData(time.Since(opStartTime), p.bufferredSize, err)
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
			for i := len(p.doneCallbacks) - 1; i >= 0; i-- {
				done := p.doneCallbacks[i]
				done()
			}

			p.Log.Debugf("Pusher published %v log events to group: %v stream: %v with size %v KB in %v.", len(p.events), p.Group, p.Stream, p.bufferredSize/1024, time.Since(startTime))
			p.addStats("rawSize", float64(p.bufferredSize))

			p.reset()
			p.lastSentTime = time.Now()

			return
		}

		p.Log.Debugf("errors not nil, we need tell exactly what's error")
		awsErr, ok := err.(awserr.Error)
		if !ok {
			p.Log.Errorf("Non aws error received when sending logs to %v/%v: %v. CloudWatch agent will not retry and logs will be missing!", p.Group, p.Stream, err)
			// Messages will be discarded but done callbacks not called
			p.reset()
			return
		}

		switch e := awsErr.(type) {
		case *cloudwatchlogs.ResourceNotFoundException:
			err := p.createLogGroupAndStream()
			if err != nil {
				p.Log.Errorf("Unable to create log stream %v/%v: %v", p.Group, p.Stream, e.Message())
				break
			}
			p.putRetentionPolicy()
		case *cloudwatchlogs.InvalidSequenceTokenException:
			if p.sequenceToken == nil {
				p.Log.Infof("First time sending logs to %v/%v since startup so sequenceToken is nil, learned new token:(%v): %v", p.Group, p.Stream, e.ExpectedSequenceToken, e.Message())
			} else {
				p.Log.Warnf("Invalid SequenceToken used (%v) while sending logs to %v/%v, will use new token and retry: %v", p.sequenceToken, p.Group, p.Stream, e.Message())
			}
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

		p.Log.Debugf("retry error detected, start retry")

		if retryCount >= 1 {
			p.Log.Debugf("ready to enqueue")
			retryinput := retryStruct{}
			retryinput.PutLogEventsInput = Clone(input)
			//inputJson, ok := json.Marshal(input)
			//p.Log.Debugf("marshal happens")
			//if ok != nil {
			//	p.Log.Debugf("marshal error happens")
			//	ok = nil
			//}
			//var temp *cloudwatchlogs.PutLogEventsInput
			//ok = json.Unmarshal(inputJson, temp)
			//retryinput.PutLogEventsInput = temp
			//p.Log.Debugf("unmarshal happens")
			//if ok != nil {
			//	p.Log.Debugf("%v", ok)
			//	p.Log.Debugf("when enqueue unmarshal errors happens")
			//}
			retryinput.FirstRetryTime = time.Now()
			retryinput.BufferredSize = p.bufferredSize
			p.wg.Add(1)
			//p.Log.Debugf("why cannot write into disk")
			go func() {
				defer p.wg.Done()
				p.Log.Debugf("start to enqueue")
				err := p.backlogQueue.Enqueue(retryinput)
				if err != nil {
					p.Log.Debugf("enqueue errors:%v", err)
				}
			}()
			for i := len(p.doneCallbacks) - 1; i >= 0; i-- {
				done := p.doneCallbacks[i]
				done()
			}
			p.addStats("rawSize", float64(p.bufferredSize))
			p.reset()
			return
		}
		p.Log.Debugf("retry happens")

		wait := retryWait(retryCount)
		if time.Since(startTime)+wait > p.RetryDuration {
			p.Log.Errorf("All %v retries to %v/%v failed for PutLogEvents, request dropped.", retryCount, p.Group, p.Stream)
			p.reset()
			return
		}

		p.Log.Warnf("Retried %v time, going to sleep %v before retrying.", retryCount, wait)

		select {
		case <-p.stop:
			p.Log.Errorf("Stop requested after %v retries to %v/%v failed for PutLogEvents, request dropped.", retryCount, p.Group, p.Stream)
			p.reset()
			return
		case <-time.After(wait):
		}

		retryCount++

	}

}
func Clone(input *cloudwatchlogs.PutLogEventsInput) *cloudwatchlogs.PutLogEventsInput {
	clone := cloudwatchlogs.PutLogEventsInput{
		LogEvents:     make([]*cloudwatchlogs.InputLogEvent, len(input.LogEvents)),
		LogGroupName:  input.LogGroupName,
		LogStreamName: input.LogStreamName,
		SequenceToken: input.SequenceToken,
	}
	for i, p := range input.LogEvents {
		if p == nil {
			continue
		}
		clone.LogEvents[i] = p
	}
	return &clone

}

func retryWait(n int) time.Duration {
	const base = 200 * time.Millisecond
	// Max wait time is 1 minute (jittered)
	d := 1 * time.Minute
	if n < 5 {
		d = base * time.Duration(1<<int64(n))
	}
	return time.Duration(seededRand.Int63n(int64(d/2)) + int64(d/2))
}

func (p *pusher) createLogGroupAndStream() error {
	_, err := p.Service.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  &p.Group,
		LogStreamName: &p.Stream,
	})

	if err == nil {
		p.Log.Debugf("successfully created log stream %v", p.Stream)
		return nil
	}

	p.Log.Debugf("creating stream fail due to : %v", err)
	if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceNotFoundException {
		_, err = p.Service.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
			LogGroupName: &p.Group,
		})

		// attempt to create stream again if group created successfully.
		if err == nil {
			p.Log.Debugf("successfully created log group %v. Retrying log stream %v", p.Group, p.Stream)
			_, err = p.Service.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
				LogGroupName:  &p.Group,
				LogStreamName: &p.Stream,
			})

			if err == nil {
				p.Log.Debugf("successfully created log stream %v", p.Stream)
			}
		} else {
			p.Log.Debugf("creating group fail due to : %v", err)
		}

	}

	if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceAlreadyExistsException {
		p.Log.Debugf("Resource was already created. %v\n", err)
		return nil // if the log group or log stream already exist, this is not worth returning an error for
	}

	return err
}

func (p *pusher) putRetentionPolicy() {
	if p.Retention > 0 {
		i := aws.Int64(int64(p.Retention))
		putRetentionInput := &cloudwatchlogs.PutRetentionPolicyInput{
			LogGroupName:    &p.Group,
			RetentionInDays: i,
		}
		_, err := p.Service.PutRetentionPolicy(putRetentionInput)
		if err != nil {
			// since this gets called both before we start pushing logs, and after we first attempt
			// to push a log to a non-existent log group, we don't want to dirty the log with an error
			// if the error is that the log group doesn't exist (yet).
			if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceNotFoundException {
				p.Log.Debugf("Log group %v not created yet: %v", p.Group, err)
			} else {
				p.Log.Errorf("Unable to put retention policy for log group %v: %v ", p.Group, err)
			}
		} else {
			p.Log.Debugf("successfully updated log retention policy for log group %v", p.Group)
		}
	}
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
