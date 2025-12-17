// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"bytes"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/text/encoding"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/tail"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

const (
	tailCloseThreshold = 3 * time.Second
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
	fileInode          uint64 // Inode of the file being tailed
	fileDev            uint64 // Device of the file being tailed
	timestampFn        func(string) (time.Time, string)
	enc                encoding.Encoding
	maxEventSize       int
	retentionInDays    int

	outputFn           func(logs.LogEvent)
	isMLStart          func(string) bool
	filters            []*LogFilter
	done               chan struct{}
	startTailerOnce    sync.Once
	cleanUpFns         []func()
	backpressureFdDrop bool
	buffer             chan *LogEvent
	stopOnce           sync.Once
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
	retentionInDays int,
	backpressureMode logscommon.BackpressureMode,
) *tailerSrc {
	// Capture the inode and device of the file being tailed
	var fileInode, fileDev uint64
	if autoRemoval && tailer.File() != nil {
		if stat, err := tailer.File().Stat(); err == nil {
			if sys := getInodeInfo(stat); sys != nil {
				fileInode = sys.Inode
				fileDev = sys.Dev
			}
		}
	}

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
		fileInode:          fileInode,
		fileDev:            fileDev,
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

func (ts *tailerSrc) Retention() int {
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

func (ts *tailerSrc) Entity() *cloudwatchlogs.Entity {
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
	ignoreUntilNextEvent := false

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
			} else if ts.isMLStart(text) || (!ignoreUntilNextEvent && msgBuf.Len() == 0) {
				init = text
				ignoreUntilNextEvent = false
			} else if ignoreUntilNextEvent || msgBuf.Len() >= ts.maxEventSize {
				ignoreUntilNextEvent = true
				fo.ShiftInt64(line.Offset)
				ts.tailer.ReleaseLine(line)
				continue
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
		fileToRemove := ts.tailer.Filename
		
		// If we have inode info, try to find the actual file by inode
		// This handles the case where the file was rotated
		if ts.fileInode != 0 {
			if actualPath := findFileByInode(ts.tailer.Filename, ts.fileInode, ts.fileDev); actualPath != "" {
				fileToRemove = actualPath
			}
		}
		
		if err := os.Remove(fileToRemove); err != nil {
			log.Printf("W! [logfile] Failed to auto remove file %v: %v", fileToRemove, err)
		} else {
			log.Printf("I! [logfile] Successfully removed file %v with auto_removal feature", fileToRemove)
		}
	}
	for _, clf := range ts.cleanUpFns {
		clf()
	}

	if ts.outputFn != nil {
		ts.outputFn(nil) // inform logs agent the tailer src's exit, to stop runSrcToDest
	}
}
