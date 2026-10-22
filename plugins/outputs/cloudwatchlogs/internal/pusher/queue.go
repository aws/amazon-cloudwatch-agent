// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/profiler"
)

const (
	// The minimum interval between logs warning that a log event was truncated, like warnOldTimeStampLogInterval.
	warnTruncatedLogEventInterval = 5 * time.Minute
	// truncatedLogEventsStat is the profiler stat that counts the log events whose message was over the per-event
	// limit and lost content: it looks like EMF, so it was truncated rather than split, or it needed more than
	// maxSplitChunks events, so the last of them was truncated. The rest of the message is dropped. It counts each
	// such log event once. A log event that is split without loss is not counted here.
	truncatedLogEventsStat = "truncatedLogEvents"
	// The minimum interval between logs warning that a log event was split. It has its own timer, so that frequent
	// splits do not mute the truncation warning.
	warnSplitLogEventInterval = 5 * time.Minute
	// splitLogEventsStat is the profiler stat that counts the log events whose message was over the per-event limit
	// and so was sent as two or more log events. It counts each such log event once.
	splitLogEventsStat = "splitLogEvents"
	// splitLogEventChunksStat is the profiler stat that counts the log events sent for split log events, so the
	// number of extra events is splitLogEventChunks - splitLogEvents.
	splitLogEventChunksStat = "splitLogEventChunks"
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
	stopCh       chan struct{}
	stopped      bool
	lastSentTime atomic.Value
	// The last time a truncated log event was logged. Only used by the start goroutine.
	lastTruncatedWarnTime time.Time
	// The last time a split log event was logged. Only used by the start goroutine.
	lastSplitWarnTime time.Time

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
		resetTimerCh:    make(chan struct{}, 1),
		flushTimer:      time.NewTimer(flushTimeout),
		stopCh:          make(chan struct{}),
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
	if q.stopped {
		return
	}
	close(q.stopCh)
	q.stopped = true
}

// start is the main loop for processing events and managing the queue.
func (q *queue) start() {
	defer q.wg.Done()
	mergeChan := make(chan logs.LogEvent)

	go q.merge(mergeChan)
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
			single, split := q.converter.convert(e)
			if single != nil {
				// Fast path: message fit in one event (either normally, or a single truncated EMF event).
				if single.truncated {
					// EMF over the per-event limit was truncated rather than split: content was lost silently.
					// Report it with a one-event slice; this is already the slow path.
					q.reportTruncated(e, []*logEvent{single})
				}
				if !q.batch.inTimeRange(single.timestamp) || !q.batch.hasSpace(single.eventBytes) {
					q.send()
				}
				q.batch.append(single)
			} else {
				// Split path: the message was over the per-event limit and sent as several events sharing its
				// timestamp. The last one carries the truncation flag.
				last := split[len(split)-1]
				// Report the converted timestamp, as it is the one sent to CloudWatch Logs and is never zero.
				q.reportSplit(e, last.timestamp, len(split))
				if last.truncated {
					// More than maxSplitChunks events would have been needed, so the last one was truncated.
					q.reportTruncated(e, split)
				}
				q.sendSplit(split)
			}
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

// merge merges events from both blocking and non-blocking channel
func (q *queue) merge(mergeChan chan logs.LogEvent) {
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
		case <-q.stopCh:
			return
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

// sendSplit sends the events of one split source event as an ordered sequence of single-event batches.
//
// The open batch is sent first, so that no other event of this queue is sent between the events of the sequence and
// the open batch does not wait behind them. Each event then gets a batch of its own: a full chunk fills a request by
// itself, and a batch per event keeps the sizes computed as for any batch. The batches are handed to the sender
// together (Sender.SendOrdered), which sends each only after the one before it has completed, with or without a
// worker pool. The last event carries the source event's state and done callback (see newStatefulLogEvents), so the
// file offset advances only after the batches of all the events have completed.
func (q *queue) sendSplit(events []*logEvent) {
	q.send()
	batches := make([]*logEventBatch, 0, len(events))
	for _, event := range events {
		// Use the entity provider of the open batch (always q.entityProvider), so that the events get the entity an
		// event appended to it would get.
		batch := newLogEventBatch(q.target, q.entityProvider)
		batch.append(event)
		batch.addDoneCallback(q.onSuccessCallback(batch.bufferedSize))
		batches = append(batches, batch)
	}
	q.sender.SendOrdered(batches)
}

// onSuccessCallback returns a callback function to be executed after a successful send.
func (q *queue) onSuccessCallback(bufferedSize int) func() {
	return func() {
		q.lastSentTime.Store(time.Now())
		go q.addStats("rawSize", float64(bufferedSize))
		q.resetFlushTimer()
	}
}

// reportTruncated counts a log event whose message was truncated, and logs a warning about it at most once per
// warnTruncatedLogEventInterval. The stat is added before the throttle check, so every truncated event is counted.
// events are the events the log event was converted to; the last one is the truncated one.
func (q *queue) reportTruncated(e logs.LogEvent, events []*logEvent) {
	q.addStats(truncatedLogEventsStat, 1)
	now := time.Now()
	if now.Sub(q.lastTruncatedWarnTime) <= warnTruncatedLogEventInterval {
		return
	}
	q.lastTruncatedWarnTime = now
	// The original message is measured again only when the warning is logged, which keeps the truncation path cheap.
	// The limit is the one for the message alone: the agent reserves perEventHeaderBytes of maxEventPayloadBytes for
	// each event.
	size := utf8EncodedLength(e.Message())
	var outcome string
	if len(events) == 1 {
		// A message that is not split but truncated: only one that looks like EMF, as maxSplitChunks is more than 1.
		outcome = "it looks like embedded metric format, so it was truncated, not split, and the remainder dropped."
	} else {
		// The counted bytes kept are those of the events sent, less the suffix added to the last one.
		kept := -len(defaultTruncationSuffix)
		for _, event := range events {
			kept += event.eventBytes - perEventHeaderBytes
		}
		outcome = fmt.Sprintf("the message was sent as %d log events and the remaining %d counted bytes dropped.", len(events), size-kept)
	}
	q.logger.Warnf("The log entry in (%v/%v) with timestamp (%v) has a message counted as %d bytes by CloudWatch Logs (%d raw bytes; each byte that is not valid UTF-8 counts as %d), which exceeds the %d byte limit per log event message (after the agent's %d byte per-event overhead); %s",
		q.target.Group, q.target.Stream, events[len(events)-1].timestamp, size, len(e.Message()), invalidUTF8ByteSize, maxEventMessageBytes, perEventHeaderBytes, outcome)
}

// reportSplit counts a log event whose message was sent as numEvents log events, and logs a warning about it at most
// once per warnSplitLogEventInterval, on a timer of its own. The stats are added before the throttle check, so every
// split event is counted.
func (q *queue) reportSplit(e logs.LogEvent, timestamp time.Time, numEvents int) {
	q.addStats(splitLogEventsStat, 1)
	q.addStats(splitLogEventChunksStat, float64(numEvents))
	now := time.Now()
	if now.Sub(q.lastSplitWarnTime) <= warnSplitLogEventInterval {
		return
	}
	q.lastSplitWarnTime = now
	// As in reportTruncated, the original message is measured again only when the warning is logged.
	q.logger.Warnf("The log entry in (%v/%v) with timestamp (%v) has a message counted as %d bytes by CloudWatch Logs (%d raw bytes; each byte that is not valid UTF-8 counts as %d), which exceeds the %d byte limit per log event message (after the agent's %d byte per-event overhead); it was sent as %d log events with the same timestamp.",
		q.target.Group, q.target.Stream, timestamp, utf8EncodedLength(e.Message()), len(e.Message()), invalidUTF8ByteSize, maxEventMessageBytes, perEventHeaderBytes, numEvents)
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
		case <-q.stopCh:
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
