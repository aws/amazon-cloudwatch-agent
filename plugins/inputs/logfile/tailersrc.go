// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"bytes"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/text/encoding"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/plugins/inputs/logfile/tail"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

const (
	stateFileMode       = 0644
	closeReopenInterval = 100 * time.Millisecond
)

var (
	multilineWaitPeriod = 1 * time.Second
	defaultBufferSize   = 1
)

type fileOffset struct {
	seq, offset int64 // Seq handles file trucation, when file is trucated, we increase the offset seq
}

func (fo *fileOffset) SetOffset(o int64) {
	if o < fo.offset { // Increment the sequence number when a smaller offset is given (truncated)
		fo.seq++
	}
	fo.offset = o
}

type LogEvent struct {
	msg    string
	t      time.Time
	offset fileOffset
	src    *tailerSrc
}

func (le LogEvent) Message() string {
	return le.msg
}

func (le LogEvent) Time() time.Time {
	return le.t
}

func (le LogEvent) Done() {
	le.src.Done(le.offset)
}

type tailerSrc struct {
	group           string
	stream          string
	class           string
	fileGlobPath    string
	destination     string
	stateFilePath   string
	tailer          *tail.Tail
	autoRemoval     bool
	timestampFn     func(string) (time.Time, string)
	enc             encoding.Encoding
	maxEventSize    int
	truncateSuffix  string
	retentionInDays int

	outputFn         func(logs.LogEvent)
	isMLStart        func(string) bool
	filters          []*LogFilter
	offsetCh         chan fileOffset
	done             chan struct{}
	startTailerOnce  sync.Once
	cleanUpFns       []func()
	backpressureDrop bool
	buffer           chan *LogEvent
	stopOnce         sync.Once
	bufferCloseOnce  sync.Once
	outputFnMu       sync.RWMutex // mutex to protect outputFn access
}

// Verify tailerSrc implements LogSrc
var _ logs.LogSrc = (*tailerSrc)(nil)

func NewTailerSrc(
	group, stream, destination, stateFilePath, logClass, fileGlobPath string,
	tailer *tail.Tail,
	autoRemoval bool,
	isMultilineStartFn func(string) bool,
	filters []*LogFilter,
	timestampFn func(string) (time.Time, string),
	enc encoding.Encoding,
	maxEventSize int,
	truncateSuffix string,
	retentionInDays int,
	backpressureDrop bool,
) *tailerSrc {
	useBufferedSender := backpressureDrop && !autoRemoval
	ts := &tailerSrc{
		group:            group,
		stream:           stream,
		destination:      destination,
		stateFilePath:    stateFilePath,
		class:            logClass,
		fileGlobPath:     fileGlobPath,
		tailer:           tailer,
		autoRemoval:      autoRemoval,
		isMLStart:        isMultilineStartFn,
		filters:          filters,
		timestampFn:      timestampFn,
		enc:              enc,
		maxEventSize:     maxEventSize,
		truncateSuffix:   truncateSuffix,
		retentionInDays:  retentionInDays,
		backpressureDrop: useBufferedSender,

		offsetCh:        make(chan fileOffset, 2000),
		done:            make(chan struct{}),
		stopOnce:        sync.Once{},
		bufferCloseOnce: sync.Once{},
	}

	if useBufferedSender {
		ts.buffer = make(chan *LogEvent, defaultBufferSize)
	}
	go ts.runSaveState()
	return ts
}

func (ts *tailerSrc) SetOutput(fn func(logs.LogEvent)) {
	if fn == nil {
		return
	}
	ts.outputFn = fn
	ts.startTailerOnce.Do(func() {
		go ts.runTail()
		if ts.backpressureDrop {
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
func (ts *tailerSrc) Done(offset fileOffset) {
	// ts.offsetCh will only be blocked when the runSaveState func has exited,
	// which only happens when the original file has been removed, thus making
	// Keeping its offset useless
	select {
	case ts.offsetCh <- offset:
	default:
	}
}

func (ts *tailerSrc) Stop() {
	ts.stopOnce.Do(func() {
		close(ts.done)
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
	fo := &fileOffset{}
	ignoreUntilNextEvent := false

	for {
		select {
		case line, ok := <-ts.tailer.Lines:
			if !ok {
				ts.publishEvent(msgBuf, fo)
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
				msgBuf.Reset()
				msgBuf.WriteString(text)
				fo.SetOffset(line.Offset)
				init = ""
			} else if ts.isMLStart(text) || (!ignoreUntilNextEvent && msgBuf.Len() == 0) {
				init = text
				ignoreUntilNextEvent = false
			} else if ignoreUntilNextEvent || msgBuf.Len() >= ts.maxEventSize {
				ignoreUntilNextEvent = true
				fo.SetOffset(line.Offset)
				continue
			} else {
				msgBuf.WriteString("\n")
				msgBuf.WriteString(text)
				if msgBuf.Len() > ts.maxEventSize {
					msgBuf.Truncate(ts.maxEventSize - len(ts.truncateSuffix))
					msgBuf.WriteString(ts.truncateSuffix)
				}
				fo.SetOffset(line.Offset)
				continue
			}

			ts.publishEvent(msgBuf, fo)
			msgBuf.Reset()
			msgBuf.WriteString(init)
			fo.SetOffset(line.Offset)
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

func (ts *tailerSrc) publishEvent(msgBuf bytes.Buffer, fo *fileOffset) {
	// helper to handle event publishing
	if msgBuf.Len() == 0 {
		return
	}
	msg := msgBuf.String()
	timestamp, modifiedMsg := ts.timestampFn(msg)
	e := &LogEvent{
		msg:    modifiedMsg,
		t:      timestamp,
		offset: *fo,
		src:    ts,
	}
	if ShouldPublish(ts.group, ts.stream, ts.filters, e) {
		if ts.backpressureDrop {
			select {
			case ts.buffer <- e:
				// successfully sent
			case <-ts.done:
				return
			default:
				// Buffer is full, close file and try again
				log.Printf("D! [logfile] tailer sender buffer is full, closing file %v", ts.tailer.Filename)
				ts.tailer.CloseFile()

				for {
					select {
					case ts.buffer <- e:
						// reopen the file after successful send
						if err := ts.tailer.Reopen(false); err != nil {
							log.Printf("E! [logfile] error reopening file %s: %v", ts.tailer.Filename, err)
							return
						}
						return
					case <-ts.done:
						return
					default:
						// buffer still full, sleep and try again
						select {
						case <-time.After(closeReopenInterval):
						case <-ts.done:
							return
						}
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
	defer func() {
		ts.bufferCloseOnce.Do(func() {
			if ts.buffer != nil {
				close(ts.buffer)
			}
		})
		log.Printf("D! [logfile] runSender exiting for %s", ts.tailer.Filename)
	}()

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
					ts.outputFnMu.RLock()
					if ts.outputFn != nil {
						ts.outputFn(e)
					}
					ts.outputFnMu.RUnlock()
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

	ts.outputFnMu.Lock()
	if ts.outputFn != nil {
		ts.outputFn(nil)
		ts.outputFn = nil
	}
	ts.outputFnMu.Unlock()
}

func (ts *tailerSrc) runSaveState() {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()

	var offset, lastSavedOffset fileOffset
	for {
		select {
		case o := <-ts.offsetCh:
			if o.seq > offset.seq || (o.seq == offset.seq && o.offset > offset.offset) {
				offset = o
			}
		case <-t.C:
			if offset == lastSavedOffset {
				continue
			}
			err := ts.saveState(offset.offset)
			if err != nil {
				log.Printf("E! [logfile] Error happened when saving file state %s to file state folder %s: %v", ts.tailer.Filename, ts.stateFilePath, err)
				continue
			}
			lastSavedOffset = offset
		case <-ts.tailer.FileDeletedCh:
			log.Printf("W! [logfile] deleting state file %s", ts.stateFilePath)
			err := os.Remove(ts.stateFilePath)
			if err != nil {
				log.Printf("W! [logfile] Error happened while deleting state file %s on cleanup: %v", ts.stateFilePath, err)
			}
			return
		case <-ts.done:
			err := ts.saveState(offset.offset)
			if err != nil {
				log.Printf("E! [logfile] Error happened during final file state saving of logfile %s to file state folder %s, duplicate log maybe sent at next start: %v", ts.tailer.Filename, ts.stateFilePath, err)
			}
			return
		}
	}
}

func (ts *tailerSrc) saveState(offset int64) error {
	if ts.stateFilePath == "" || offset == 0 {
		return nil
	}

	content := []byte(strconv.FormatInt(offset, 10) + "\n" + ts.tailer.Filename)
	return os.WriteFile(ts.stateFilePath, content, stateFileMode)
}
