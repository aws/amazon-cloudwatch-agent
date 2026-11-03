// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"bytes"
	"log"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"golang.org/x/text/encoding"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/tail"
	"github.com/aws/amazon-cloudwatch-agent/profiler"
)

const (
	tailCloseThreshold = 3 * time.Second
	// The minimum interval between logs that a multi-line group was split into several events, per tailer.
	warnMultiLineSplitInterval = 5 * time.Minute
)

var (
	multilineWaitPeriod = 1 * time.Second
	defaultBufferSize   = 1
)

type LogEvent struct {
	msg    string
	t      time.Time
	offset state.Range
	src    *tailerSrc
}

var _ logs.StatefulLogEvent = (*LogEvent)(nil)

func (le LogEvent) Message() string {
	return le.msg
}

func (le LogEvent) Time() time.Time {
	return le.t
}

func (le LogEvent) Done() {
	le.RangeQueue().Enqueue(le.Range())
}

func (le LogEvent) Range() state.Range {
	return le.offset
}

func (le LogEvent) RangeQueue() state.FileRangeQueue {
	return le.src.stateManager
}

type tailerSrc struct {
	group              string
	stream             string
	class              string
	fileGlobPath       string
	destination        string
	stateManager       state.FileRangeManager
	initialStateOffset int64
	tailer             *tail.Tail
	autoRemoval        bool
	timestampFn        func(string) (time.Time, string)
	enc                encoding.Encoding
	maxEventSize       int
	retentionInDays    int32

	outputFn           func(logs.LogEvent)
	isMLStart          func(string) bool
	filters            []*LogFilter
	done               chan struct{}
	startTailerOnce    sync.Once
	cleanUpFns         []func()
	backpressureFdDrop bool
	buffer             chan *LogEvent
	stopOnce           sync.Once
	// The last time a split multi-line group was logged. Only used by the runTail goroutine.
	lastMultiLineSplitWarnTime time.Time
}

// Verify tailerSrc implements LogSrc
var _ logs.LogSrc = (*tailerSrc)(nil)

func NewTailerSrc(
	group, stream, destination string,
	stateManager state.FileRangeManager,
	initialStateOffset int64,
	logClass, fileGlobPath string,
	tailer *tail.Tail,
	autoRemoval bool,
	isMultilineStartFn func(string) bool,
	filters []*LogFilter,
	timestampFn func(string) (time.Time, string),
	enc encoding.Encoding,
	maxEventSize int,
	retentionInDays int32,
	backpressureMode logscommon.BackpressureMode,
) *tailerSrc {
	ts := &tailerSrc{
		group:              group,
		stream:             stream,
		destination:        destination,
		stateManager:       stateManager,
		initialStateOffset: initialStateOffset,
		class:              logClass,
		fileGlobPath:       fileGlobPath,
		tailer:             tailer,
		autoRemoval:        autoRemoval,
		isMLStart:          isMultilineStartFn,
		filters:            filters,
		timestampFn:        timestampFn,
		enc:                enc,
		maxEventSize:       maxEventSize,
		retentionInDays:    retentionInDays,
		backpressureFdDrop: !autoRemoval && backpressureMode == logscommon.LogBackpressureModeFDRelease,
		done:               make(chan struct{}),
	}

	if ts.backpressureFdDrop {
		ts.buffer = make(chan *LogEvent, defaultBufferSize)
	}
	go ts.stateManager.Run(state.Notification{
		Delete: ts.tailer.FileDeletedCh,
		Done:   ts.done,
	})
	return ts
}

func (ts *tailerSrc) SetOutput(fn func(logs.LogEvent)) {
	if fn == nil {
		return
	}
	ts.outputFn = fn
	ts.startTailerOnce.Do(func() {
		go ts.runTail()
		if ts.backpressureFdDrop {
			go ts.runSender()
		}
	})
}

func (ts *tailerSrc) Group() string {
	return ts.group
}

func (ts *tailerSrc) Stream() string {
	return ts.stream
}

func (ts *tailerSrc) Description() string {
	return ts.tailer.Filename
}

func (ts *tailerSrc) Destination() string {
	return ts.destination
}

func (ts *tailerSrc) Retention() int32 {
	return ts.retentionInDays
}

func (ts *tailerSrc) Class() string {
	return ts.class
}

func (ts *tailerSrc) Stop() {
	ts.stopOnce.Do(func() {
		close(ts.done)
		if ts.buffer != nil {
			close(ts.buffer)
		}
	})
}

func (ts *tailerSrc) AddCleanUpFn(f func()) {
	ts.cleanUpFns = append(ts.cleanUpFns, f)
}

func (ts *tailerSrc) Entity() *types.Entity {
	es := entitystore.GetEntityStore()
	if es != nil {
		return es.CreateLogFileEntity(entitystore.LogFileGlob(ts.fileGlobPath), entitystore.LogGroupName(ts.group))
	}
	return nil
}

func (ts *tailerSrc) runTail() {
	defer ts.cleanUp()
	t := time.NewTicker(multilineWaitPeriod)
	defer t.Stop()
	var init string
	var msgBuf bytes.Buffer
	var cnt int
	fo := state.Range{}
	if ts.initialStateOffset > 0 {
		fo.SetInt64(0, ts.initialStateOffset)
	}

	for {
		select {
		// Warning: Make sure to release line once done!
		case line, ok := <-ts.tailer.Lines:
			if !ok {
				ts.publishEvent(msgBuf, fo)
				return
			}

			if line.Err != nil {
				log.Printf("E! [logfile] Error tailing line in file %s, Error: %s\n", ts.tailer.Filename, line.Err)
				ts.tailer.ReleaseLine(line)
				continue
			}

			text := line.Text
			if ts.enc != nil {
				var err error
				text, err = ts.enc.NewDecoder().String(text)
				if err != nil {
					log.Printf("E! [logfile] Cannot decode the log file content for %s: %v\n", ts.tailer.Filename, err)
					ts.tailer.ReleaseLine(line)
					continue
				}
			}

			if ts.isMLStart == nil {
				msgBuf.Reset()
				msgBuf.WriteString(text)
				fo.ShiftInt64(line.Offset)
				init = ""
			} else if ts.isMLStart(text) || msgBuf.Len() == 0 {
				init = text
			} else if msgBuf.Len()+1+len(text) > ts.maxEventSize {
				// Appending this continuation line ("\n" plus the line) would take the event past maxEventSize. Rather
				// than drop it and the rest of the group, publish the event so far and start a new event with this
				// line, as for a start line below. The group is cut on a line boundary and nothing is dropped. The
				// check runs before the append, so an event never grows past maxEventSize unless a single line is
				// longer than that; the cloudwatchlogs output splits such an event further if needed.
				ts.reportMultiLineSplit()
				init = text
			} else {
				msgBuf.WriteString("\n")
				msgBuf.WriteString(text)
				fo.ShiftInt64(line.Offset)
				ts.tailer.ReleaseLine(line)
				continue
			}

			ts.publishEvent(msgBuf, fo)
			msgBuf.Reset()
			msgBuf.WriteString(init)
			fo.ShiftInt64(line.Offset)
			ts.tailer.ReleaseLine(line)
			cnt = 0
		case <-t.C:
			if msgBuf.Len() > 0 {
				cnt++
			}

			if cnt >= 5 {
				ts.publishEvent(msgBuf, fo)
				msgBuf.Reset()
				cnt = 0
			}
		case <-ts.done:
			return
		}
	}
}

// reportMultiLineSplit counts a multi-line group that was split into several events because it would have exceeded
// maxEventSize, and logs it at most once per warnMultiLineSplitInterval. The stat is added before the throttle check,
// so every split is counted, with a key like the one for filtered messages (see ShouldPublish).
func (ts *tailerSrc) reportMultiLineSplit() {
	profiler.Profiler.AddStats([]string{"logfile", ts.group, ts.stream, "messages", "multiLineSplit"}, 1)
	now := time.Now()
	if now.Sub(ts.lastMultiLineSplitWarnTime) <= warnMultiLineSplitInterval {
		return
	}
	ts.lastMultiLineSplitWarnTime = now
	log.Printf("W! [logfile] A multi-line log event in file %s would exceed the max_event_size of %d bytes, so it was split into several log events at line boundaries.", ts.tailer.Filename, ts.maxEventSize)
}

func (ts *tailerSrc) publishEvent(msgBuf bytes.Buffer, fo state.Range) {
	// helper to handle event publishing
	if msgBuf.Len() == 0 {
		return
	}
	msg := msgBuf.String()
	timestamp, modifiedMsg := ts.timestampFn(msg)
	e := &LogEvent{
		msg:    modifiedMsg,
		t:      timestamp,
		offset: fo,
		src:    ts,
	}
	if ShouldPublish(ts.group, ts.stream, ts.filters, e) {
		if ts.backpressureFdDrop {
			select {
			case ts.buffer <- e:
				// successfully sent
			case <-ts.done:
				return
			default:
				// sender buffer is full. start timer to close file then retry
				timer := time.NewTimer(tailCloseThreshold)
				defer timer.Stop()

				for {
					select {
					case ts.buffer <- e:
						// sent event after buffer gets freed up
						if ts.tailer.IsFileClosed() { // skip file closing if not already closed
							if err := ts.tailer.Reopen(false); err != nil {
								log.Printf("E! [logfile] error reopening file %s: %v", ts.tailer.Filename, err)
							}
						}
						return
					case <-timer.C:
						// timer expired without successful send, close file
						log.Printf("D! [logfile] tailer sender buffer blocked after retrying, closing file %v", ts.tailer.Filename)
						ts.tailer.CloseFile()
					case <-ts.done:
						return
					}
				}
			}
		} else {
			ts.outputFn(e)
		}
	}
}

func (ts *tailerSrc) runSender() {
	log.Printf("D! [logfile] runSender starting for %s", ts.tailer.Filename)

	for {
		select {
		case e, ok := <-ts.buffer:
			if !ok { // buffer was closed
				log.Printf("D! [logfile] runSender buffer was closed for %s", ts.tailer.Filename)
				return
			}
			// Check done before sending
			select {
			case <-ts.done:
				return
			default:
				if e != nil {
					ts.outputFn(e)
				}
			}
		case <-ts.done:
			log.Printf("D! [logfile] runSender received done signal for %s", ts.tailer.Filename)
			return
		}
	}
}

func (ts *tailerSrc) cleanUp() {
	if ts.autoRemoval {
		if err := os.Remove(ts.tailer.Filename); err != nil {
			log.Printf("W! [logfile] Failed to auto remove file %v: %v", ts.tailer.Filename, err)
		} else {
			log.Printf("I! [logfile] Successfully removed file %v with auto_removal feature", ts.tailer.Filename)
		}
	}
	for _, clf := range ts.cleanUpFns {
		clf()
	}

	if ts.outputFn != nil {
		ts.outputFn(nil) // inform logs agent the tailer src's exit, to stop runSrcToDest
	}
}
