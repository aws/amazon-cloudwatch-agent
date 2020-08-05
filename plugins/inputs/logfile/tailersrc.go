// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/tail"
	"golang.org/x/text/encoding"
)

const (
	stateFileMode = 0644
	bufferLimit   = 50
)

var (
	multilineWaitPeriod = 1 * time.Second
)

type tailerSrc struct {
	group, stream  string
	destination    string
	stateFilePath  string
	tailer         *tail.Tail
	autoRemoval    bool
	timestampFn    func(string) time.Time
	enc            encoding.Encoding
	maxEventSize   int
	truncateSuffix string

	outputFn        func(logs.LogEvent)
	isMLStart       func(string) bool
	offsetCh        chan int64
	done            chan struct{}
	startTailerOnce sync.Once
	cleanUpFns      []func()
}

func NewTailerSrc(
	group, stream, destination, stateFilePath string,
	tailer *tail.Tail,
	autoRemoval bool,
	isMultilineStartFn func(string) bool,
	timestampFn func(string) time.Time,
	enc encoding.Encoding,
	maxEventSize int,
	truncateSuffix string,
) *tailerSrc {
	ts := &tailerSrc{
		group:          group,
		stream:         stream,
		destination:    destination,
		stateFilePath:  stateFilePath,
		tailer:         tailer,
		autoRemoval:    autoRemoval,
		isMLStart:      isMultilineStartFn,
		timestampFn:    timestampFn,
		enc:            enc,
		maxEventSize:   maxEventSize,
		truncateSuffix: truncateSuffix,

		offsetCh: make(chan int64, 100),
		done:     make(chan struct{}),
	}
	go ts.runSaveState()
	return ts
}

func (ts *tailerSrc) SetOutput(fn func(logs.LogEvent)) {
	if fn == nil {
		return
	}
	ts.outputFn = fn
	ts.startTailerOnce.Do(func() { go ts.runTail() })
}

func (ts tailerSrc) Group() string {
	return ts.group
}

func (ts tailerSrc) Stream() string {
	return ts.stream
}

func (ts tailerSrc) Description() string {
	return ts.tailer.Filename
}

func (ts tailerSrc) Destination() string {
	return ts.destination
}

func (ts tailerSrc) Done(offset int64) {
	ts.offsetCh <- offset
}

func (ts *tailerSrc) Stop() {
	close(ts.done)
}

func (ts *tailerSrc) AddCleanUpFn(f func()) {
	ts.cleanUpFns = append(ts.cleanUpFns, f)
}

func (ts *tailerSrc) runTail() {
	defer ts.cleanUp()
	t := time.NewTicker(multilineWaitPeriod)
	defer t.Stop()
	var msg, init string
	var cnt int
	var offset int64

	ignoreUntilNextEvent := false
	for {

		select {
		case line, ok := <-ts.tailer.Lines:
			if !ok {
				if msg != "" {
					e := &LogEvent{
						msg:    msg,
						t:      ts.timestampFn(msg),
						offset: offset,
						src:    ts,
					}
					ts.outputFn(e)
				}
				return
			}

			if line.Err != nil {
				log.Printf("E! [logfile] Error tailing line in file %s, Error: %s\n", ts.tailer.Filename, line.Err)
				continue
			}

			text := line.Text
			if ts.enc != nil {
				var err error
				text, err = ts.enc.NewDecoder().String(text)
				if err != nil {
					log.Printf("E! [logfile] Cannot decode the log file content for %s: %v\n", ts.tailer.Filename, err)
					continue
				}
			}

			if ts.isMLStart == nil {
				msg = text
				offset = line.Offset
				init = ""
			} else if ts.isMLStart(text) || (!ignoreUntilNextEvent && msg == "") {
				init = text
				offset = line.Offset
				ignoreUntilNextEvent = false
			} else if ignoreUntilNextEvent || len(msg) > ts.maxEventSize {
				ignoreUntilNextEvent = true
				continue
			} else {
				msg += "\n" + text
				offset = line.Offset
				continue
			}

			if len(msg) > ts.maxEventSize {
				msg = msg[:ts.maxEventSize-len(ts.truncateSuffix)] + ts.truncateSuffix
			}

			if msg != "" {
				e := &LogEvent{
					msg:    msg,
					t:      ts.timestampFn(msg),
					offset: offset,
					src:    ts,
				}
				ts.outputFn(e)
			}

			msg = init
			cnt = 0
		case <-t.C:
			if msg != "" {
				cnt++
			}

			if cnt < 5 {
				continue
			}

			e := &LogEvent{
				msg:    msg,
				t:      ts.timestampFn(msg),
				offset: offset,
				src:    ts,
			}
			ts.outputFn(e)
			msg = ""
			cnt = 0
		case <-ts.done:
			return
		}
	}
}

func (ts *tailerSrc) cleanUp() {
	if ts.autoRemoval {
		if err := os.Remove(ts.tailer.Filename); err != nil {
			log.Printf("W! [logfile] Failed to auto remove file %v: %v", ts.tailer.Filename, err)
		}
	}
	for _, clf := range ts.cleanUpFns {
		clf()
	}
	if ts.outputFn != nil {
		ts.outputFn(nil) // inform logs agent the tailer src's exit, to stop runSrcToDest
	}
}

func (ts *tailerSrc) runSaveState() {
	t := time.NewTicker(100 * time.Millisecond)

	var offset, lastSavedOffset int64
	for {
		select {
		case o := <-ts.offsetCh:
			if o > offset {
				offset = o
			}
		case <-t.C:
			if offset == lastSavedOffset {
				continue
			}
			err := ts.saveState(offset)
			if err != nil {
				log.Printf("E! [logfile] Error happened when saving file state %s to file state folder %s: %v", ts.tailer.Filename, ts.stateFilePath, err)
				continue
			}
			lastSavedOffset = offset
		case <-ts.done:
			err := ts.saveState(offset)
			if err != nil {
				log.Printf("E! [logfile] Error happened during final file state saving of logfile %s to file state folder %s, duplicate log maybe sent at next start: %v", ts.tailer.Filename, ts.stateFilePath, err)
			}
			break
		}
	}
}

func (ts *tailerSrc) saveState(offset int64) error {
	if ts.stateFilePath == "" || offset == 0 {
		return nil
	}

	content := []byte(strconv.FormatInt(offset, 10) + "\n" + ts.tailer.Filename)
	return ioutil.WriteFile(ts.stateFilePath, content, stateFileMode)
}
