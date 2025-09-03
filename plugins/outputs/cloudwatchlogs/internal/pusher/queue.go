// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/profiler"
)

type Queue interface {
	AddEvent(e logs.LogEvent)
	AddEventNonBlocking(e logs.LogEvent)
	Stop()
}

type queue struct {
	target Target
	logger telegraf.Logger

	entityProvider      logs.LogEntityProvider
	sender              Sender
	converter           *converter
	batch               *logEventBatch
	eventsCh            chan logs.LogEvent
	nonBlockingEventsCh chan logs.LogEvent

	flushCh      chan struct{}
	resetTimerCh chan struct{}
	flushTimer   *time.Timer
	flushTimeout atomic.Value
	stop         chan struct{}
	lastSentTime atomic.Value

	initNonBlockingChOnce sync.Once
	startNonBlockCh       chan struct{}
	wg                    *sync.WaitGroup
}

var _ (Queue) = (*queue)(nil)

func newQueue(
	logger telegraf.Logger,
	target Target,
	flushTimeout time.Duration,
	entityProvider logs.LogEntityProvider,
	sender Sender,
	wg *sync.WaitGroup,
) Queue {
	q := &queue{
		target:          target,
		logger:          logger,
		converter:       newConverter(logger, target),
		batch:           newLogEventBatch(target, entityProvider),
		sender:          sender,
		eventsCh:        make(chan logs.LogEvent, 100),
		flushCh:         make(chan struct{}),
		resetTimerCh:    make(chan struct{}),
		flushTimer:      time.NewTimer(flushTimeout),
		stop:            make(chan struct{}),
		startNonBlockCh: make(chan struct{}),
		wg:              wg,
	}
	q.flushTimeout.Store(flushTimeout)
	q.wg.Add(1)
	go q.start()
	return q
}

// AddEvent adds an event to the queue blocking if full.
func (q *queue) AddEvent(e logs.LogEvent) {
	if !hasValidTime(e) {
		q.logger.Errorf("The log entry in (%v/%v) with timestamp (%v) comparing to the current time (%v) is out of accepted time range. Discard the log entry.", q.target.Group, q.target.Stream, e.Time(), time.Now())
		return
	}
	q.eventsCh <- e
}

// AddEventNonBlocking adds an event to the queue without blocking. If the queue is full, drops the oldest event in
// the queue.
func (q *queue) AddEventNonBlocking(e logs.LogEvent) {
	if !hasValidTime(e) {
		q.logger.Errorf("The log entry in (%v/%v) with timestamp (%v) comparing to the current time (%v) is out of accepted time range. Discard the log entry.", q.target.Group, q.target.Stream, e.Time(), time.Now())
		return
	}

	q.initNonBlockingChOnce.Do(func() {
		q.nonBlockingEventsCh = make(chan logs.LogEvent, reqEventsLimit*2)
		q.startNonBlockCh <- struct{}{} // Unblock the select loop to recognize the channel merge
	})

	// Drain the channel until new event can be added
	for {
		select {
		case q.nonBlockingEventsCh <- e:
			return
		default:
			<-q.nonBlockingEventsCh
			q.addStats("emfMetricDrop", 1)
		}
	}
}

// Stop stops all goroutines associated with this queue instance.
func (q *queue) Stop() {
	close(q.stop)
}

// start is the main loop for processing events and managing the queue.
func (q *queue) start() {
	defer q.wg.Done()
	mergeChan := make(chan logs.LogEvent)

	// Merge events from both blocking and non-blocking channel
	go func() {
		defer close(mergeChan)
		var nonBlockingEventsCh <-chan logs.LogEvent
		for {
			select {
			case e := <-q.eventsCh:
				mergeChan <- e
			case e := <-nonBlockingEventsCh:
				mergeChan <- e
			case <-q.startNonBlockCh:
				nonBlockingEventsCh = q.nonBlockingEventsCh
			case <-q.stop:
				return
			}
		}
	}()

	go q.manageFlushTimer()

	for {
		select {
		case e, ok := <-mergeChan:
			if !ok {
				q.send()
				return
			}
			// Start timer when first event of the batch is added (happens after a flush timer timeout)
			if len(q.batch.events) == 0 {
				q.resetFlushTimer()
			}
			event := q.converter.convert(e)
			if !q.batch.inTimeRange(event.timestamp) || !q.batch.hasSpace(event.eventBytes) {
				q.send()
			}
			q.batch.append(event)
		case <-q.flushCh:
			lastSentTime, _ := q.lastSentTime.Load().(time.Time)
			flushTimeout, _ := q.flushTimeout.Load().(time.Duration)
			if time.Since(lastSentTime) >= flushTimeout && len(q.batch.events) > 0 {
				q.send()
			} else {
				q.resetFlushTimer()
			}
		}
	}
}

// send the current batch of events.
func (q *queue) send() {
	if len(q.batch.events) > 0 {
		q.batch.addDoneCallback(q.onSuccessCallback(q.batch.bufferedSize))
		q.sender.Send(q.batch)
		q.batch = newLogEventBatch(q.target, q.entityProvider)
	}
}

// onSuccessCallback returns a callback function to be executed after a successful send.
func (q *queue) onSuccessCallback(bufferedSize int) func() {
	return func() {
		q.lastSentTime.Store(time.Now())
		go q.addStats("rawSize", float64(bufferedSize))
		q.resetFlushTimer()
	}
}

// addStats adds statistics to the profiler.
func (q *queue) addStats(statsName string, value float64) {
	statsKey := []string{"cloudwatchlogs", q.target.Group, statsName}
	profiler.Profiler.AddStats(statsKey, value)
}

// manageFlushTimer manages the flush timer for the queue. Needed since the timer Stop/Reset functions cannot
// be called concurrently.
func (q *queue) manageFlushTimer() {
	for {
		select {
		case <-q.flushTimer.C:
			q.flushCh <- struct{}{}
		case <-q.resetTimerCh:
			q.stopFlushTimer()
			if flushTimeout, ok := q.flushTimeout.Load().(time.Duration); ok {
				q.flushTimer.Reset(flushTimeout)
			}
		case <-q.stop:
			q.stopFlushTimer()
			return
		}
	}
}

// stopFlushTimer stops the timer and attempts to drain it.
func (q *queue) stopFlushTimer() {
	if !q.flushTimer.Stop() {
		select {
		case <-q.flushTimer.C:
		default:
		}
	}
}

// resetFlushTimer sends a reset timer request if there isn't already one pending.
func (q *queue) resetFlushTimer() {
	select {
	case q.resetTimerCh <- struct{}{}:
	default:
	}
}

func hasValidTime(e logs.LogEvent) bool {
	//http://docs.aws.amazon.com/goto/SdkForGoV1/logs-2014-03-28/PutLogEvents
	//* None of the log events in the logEventBatch can be more than 2 hours in the future.
	//* None of the log events in the logEventBatch can be older than 14 days or the retention period of the log group.
	if !e.Time().IsZero() {
		now := time.Now()
		dt := now.Sub(e.Time()).Hours()
		if dt > 24*14 || dt < -2 {
			return false
		}
	}
	return true
}
