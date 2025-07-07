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

// Helper function for max of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

const (
	tailCloseThreshold = 3 * time.Second
	// Reserve space for CloudWatch Logs service headers and metadata
	// Based on actual CloudWatch Logs API specification:
	// - Per-event header: 52 bytes (timestamp + metadata)
	// - Additional safety margin for log group/stream names and service overhead
	// Reduced from 8KB to 1KB to prevent over-aggressive truncation
	cloudWatchHeaderReserve = 1024 * 8 // 1KB reserve (was 8KB - too aggressive)

	// Minimum content size to prevent over-truncation
	// Ensures we don't create log entries that are too small to be meaningful
	minContentSize = 100 // Minimum 100 bytes of actual content after truncation
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
}

func (le LogEvent) Range() state.Range {
	return le.offset
}

func (le LogEvent) RangeQueue() state.FileRangeQueue {
	return le.src.stateManager
}

type tailerSrc struct {
	group           string
	stream          string
	class           string
	fileGlobPath    string
	destination     string
	stateManager    state.FileRangeManager
	tailer          *tail.Tail
	autoRemoval     bool
	timestampFn     func(string) (time.Time, string)
	enc             encoding.Encoding
	maxEventSize    int
	truncateSuffix  string
	retentionInDays int

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
	logClass, fileGlobPath string,
	tailer *tail.Tail,
	autoRemoval bool,
	isMultilineStartFn func(string) bool,
	filters []*LogFilter,
	timestampFn func(string) (time.Time, string),
	enc encoding.Encoding,
	maxEventSize int,
	truncateSuffix string,
	retentionInDays int,
	backpressureMode logscommon.BackpressureMode,
) *tailerSrc {
	ts := &tailerSrc{
		group:              group,
		stream:             stream,
		destination:        destination,
		stateManager:       stateManager,
		class:              logClass,
		fileGlobPath:       fileGlobPath,
		tailer:             tailer,
		autoRemoval:        autoRemoval,
		isMLStart:          isMultilineStartFn,
		filters:            filters,
		timestampFn:        timestampFn,
		enc:                enc,
		maxEventSize:       maxEventSize,
		truncateSuffix:     truncateSuffix,
		retentionInDays:    retentionInDays,
		backpressureFdDrop: !autoRemoval && backpressureMode == logscommon.LogBackpressureModeFDRelease,
		done:               make(chan struct{}),
	}

	// Validate header reserve configuration on first tailer creation
	validator := NewHeaderReserveValidator()
	validator.MaxEventSize = maxEventSize
	validator.TruncationSuffix = truncateSuffix
	validator.ValidateConfiguration()
	validator.TestTruncationScenarios()

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

				messageSize := msgBuf.Len()

				// Handle single line larger than max event size
				if messageSize > ts.maxEventSize {
					originalSize := messageSize
					// Calculate effective max size with improved logic
					baseReserve := cloudWatchHeaderReserve + len(ts.truncateSuffix)
					effectiveMaxSize := ts.maxEventSize - baseReserve

					// Enhanced safety check to prevent over-truncation
					if effectiveMaxSize < minContentSize {
						log.Printf("[TRUNCATION ERROR] Calculated effective size too small:")
						log.Printf("  - File: %s", ts.tailer.Filename)
						log.Printf("  - Max event size: %d bytes", ts.maxEventSize)
						log.Printf("  - Base reserve (header + suffix): %d bytes", baseReserve)
						log.Printf("  - Calculated effective size: %d bytes", effectiveMaxSize)
						log.Printf("  - Minimum content size: %d bytes", minContentSize)
						log.Printf("  - Adjusting to minimum content size")

						// Adjust to ensure minimum content size
						effectiveMaxSize = minContentSize
						if effectiveMaxSize+len(ts.truncateSuffix) > ts.maxEventSize {
							// If even minimum content + suffix exceeds limit, use maximum possible
							effectiveMaxSize = ts.maxEventSize - len(ts.truncateSuffix)
							log.Printf("  - Final adjusted effective size: %d bytes", effectiveMaxSize)
						}
					}

					// Enhanced logging for truncation debugging
					log.Printf("[TRUNCATION DEBUG] Single line truncation detected:")
					log.Printf("  - File: %s", ts.tailer.Filename)
					log.Printf("  - Original message size: %d bytes (%.2f KB)", originalSize, float64(originalSize)/1024)
					log.Printf("  - Max event size limit: %d bytes (%.2f KB)", ts.maxEventSize, float64(ts.maxEventSize)/1024)
					log.Printf("  - CloudWatch header reserve: %d bytes", cloudWatchHeaderReserve)
					log.Printf("  - Truncation suffix: '%s' (%d bytes)", ts.truncateSuffix, len(ts.truncateSuffix))
					log.Printf("  - Base reserve total: %d bytes", baseReserve)
					log.Printf("  - Effective max size: %d bytes (%.2f KB)", effectiveMaxSize, float64(effectiveMaxSize)/1024)
					log.Printf("  - Bytes being truncated: %d bytes (%.1f%%)", originalSize-effectiveMaxSize, float64(originalSize-effectiveMaxSize)/float64(originalSize)*100)
					log.Printf("  - Original message preview (first 200 chars): %.200s", msgBuf.String())

					// Perform truncation with validation
					if effectiveMaxSize > 0 && effectiveMaxSize < msgBuf.Len() {
						msgBuf.Truncate(effectiveMaxSize)
						msgBuf.WriteString(ts.truncateSuffix)

						log.Printf("  - Final message size after truncation: %d bytes", msgBuf.Len())
						log.Printf("  - Final message preview (first 100 chars): %.100s", msgBuf.String())
						log.Printf("  - Final message preview (last 100 chars): %s", msgBuf.String()[max(0, msgBuf.Len()-100):])

						// Validation check
						if msgBuf.Len() > ts.maxEventSize {
							log.Printf("[TRUNCATION ERROR] Final message size exceeds limit!")
							log.Printf("  - Final size: %d bytes", msgBuf.Len())
							log.Printf("  - Max allowed: %d bytes", ts.maxEventSize)
						}
					} else {
						log.Printf("[TRUNCATION ERROR] Invalid effective size calculation: %d", effectiveMaxSize)
					}
				} else if messageSize > 200*1024 { // Log large messages that don't get truncated (>200KB)
					log.Printf("[SIZE DEBUG] Large single line message processed successfully:")
					log.Printf("  - File: %s", ts.tailer.Filename)
					log.Printf("  - Message size: %d bytes (%.1f KB)", messageSize, float64(messageSize)/1024)
					log.Printf("  - Max event size limit: %d bytes (%.1f KB)", ts.maxEventSize, float64(ts.maxEventSize)/1024)
					log.Printf("  - Remaining capacity: %d bytes", ts.maxEventSize-messageSize)
					log.Printf("  - CloudWatch Logs 256KB limit: %s", func() string {
						if messageSize <= 256*1024 {
							return "WITHIN LIMIT"
						}
						return "EXCEEDS LIMIT"
					}())
				}
				fo.ShiftInt64(line.Offset)
				init = ""
			} else if ts.isMLStart(text) || (!ignoreUntilNextEvent && msgBuf.Len() == 0) {
				init = text
				ignoreUntilNextEvent = false
			} else if ignoreUntilNextEvent || msgBuf.Len() >= ts.maxEventSize-cloudWatchHeaderReserve {
				if !ignoreUntilNextEvent {
					// First time hitting the threshold - log it
					log.Printf("[SIZE DEBUG] Multiline message approaching size limit, ignoring additional lines:")
					log.Printf("  - File: %s", ts.tailer.Filename)
					log.Printf("  - Current message size: %d bytes (%.1f KB)", msgBuf.Len(), float64(msgBuf.Len())/1024)
					log.Printf("  - Size threshold: %d bytes", ts.maxEventSize-cloudWatchHeaderReserve)
					log.Printf("  - Max event size limit: %d bytes (%.1f KB)", ts.maxEventSize, float64(ts.maxEventSize)/1024)
				}
				ignoreUntilNextEvent = true
				fo.ShiftInt64(line.Offset)
				continue
			} else {
				msgBuf.WriteString("\n")
				msgBuf.WriteString(text)

				messageSize := msgBuf.Len()

				if messageSize > ts.maxEventSize {
					originalSize := messageSize
					// Calculate effective max size with improved logic
					baseReserve := cloudWatchHeaderReserve + len(ts.truncateSuffix)
					effectiveMaxSize := ts.maxEventSize - baseReserve

					// Enhanced safety check to prevent over-truncation
					if effectiveMaxSize < minContentSize {
						log.Printf("[TRUNCATION ERROR] Multiline effective size too small:")
						log.Printf("  - File: %s", ts.tailer.Filename)
						log.Printf("  - Max event size: %d bytes", ts.maxEventSize)
						log.Printf("  - Base reserve (header + suffix): %d bytes", baseReserve)
						log.Printf("  - Calculated effective size: %d bytes", effectiveMaxSize)
						log.Printf("  - Minimum content size: %d bytes", minContentSize)
						log.Printf("  - Adjusting to minimum content size")

						// Adjust to ensure minimum content size
						effectiveMaxSize = minContentSize
						if effectiveMaxSize+len(ts.truncateSuffix) > ts.maxEventSize {
							// If even minimum content + suffix exceeds limit, use maximum possible
							effectiveMaxSize = ts.maxEventSize - len(ts.truncateSuffix)
							log.Printf("  - Final adjusted effective size: %d bytes", effectiveMaxSize)
						}
					}

					// Enhanced logging for multiline truncation debugging
					log.Printf("[TRUNCATION DEBUG] Multiline truncation detected:")
					log.Printf("  - File: %s", ts.tailer.Filename)
					log.Printf("  - Original multiline message size: %d bytes (%.2f KB)", originalSize, float64(originalSize)/1024)
					log.Printf("  - Max event size limit: %d bytes (%.2f KB)", ts.maxEventSize, float64(ts.maxEventSize)/1024)
					log.Printf("  - CloudWatch header reserve: %d bytes", cloudWatchHeaderReserve)
					log.Printf("  - Truncation suffix: '%s' (%d bytes)", ts.truncateSuffix, len(ts.truncateSuffix))
					log.Printf("  - Base reserve total: %d bytes", baseReserve)
					log.Printf("  - Effective max size: %d bytes (%.2f KB)", effectiveMaxSize, float64(effectiveMaxSize)/1024)
					log.Printf("  - Bytes being truncated: %d bytes (%.1f%%)", originalSize-effectiveMaxSize, float64(originalSize-effectiveMaxSize)/float64(originalSize)*100)
					log.Printf("  - Original multiline message preview (first 200 chars): %.200s", msgBuf.String())

					// Perform truncation with validation
					if effectiveMaxSize > 0 && effectiveMaxSize < msgBuf.Len() {
						msgBuf.Truncate(effectiveMaxSize)
						msgBuf.WriteString(ts.truncateSuffix)

						log.Printf("  - Final message size after truncation: %d bytes", msgBuf.Len())
						log.Printf("  - Final message preview (first 100 chars): %.100s", msgBuf.String())
						log.Printf("  - Final message preview (last 100 chars): %s", msgBuf.String()[max(0, msgBuf.Len()-100):])

						// Validation check
						if msgBuf.Len() > ts.maxEventSize {
							log.Printf("[TRUNCATION ERROR] Final multiline message size exceeds limit!")
							log.Printf("  - Final size: %d bytes", msgBuf.Len())
							log.Printf("  - Max allowed: %d bytes", ts.maxEventSize)
						}
					} else {
						log.Printf("[TRUNCATION ERROR] Invalid multiline effective size calculation: %d", effectiveMaxSize)
					}
					log.Printf("  - Final message preview (last 200 chars): %s", msgBuf.String()[max(0, msgBuf.Len()-200):])
				} else if messageSize > 200*1024 { // Log large multiline messages that don't get truncated (>200KB)
					log.Printf("[SIZE DEBUG] Large multiline message processed successfully:")
					log.Printf("  - File: %s", ts.tailer.Filename)
					log.Printf("  - Message size: %d bytes (%.1f KB)", messageSize, float64(messageSize)/1024)
					log.Printf("  - Max event size limit: %d bytes (%.1f KB)", ts.maxEventSize, float64(ts.maxEventSize)/1024)
					log.Printf("  - Remaining capacity: %d bytes", ts.maxEventSize-messageSize)
					log.Printf("  - CloudWatch Logs 256KB limit: %s", func() string {
						if messageSize <= 256*1024 {
							return "WITHIN LIMIT"
						}
						return "EXCEEDS LIMIT"
					}())
				}
				fo.ShiftInt64(line.Offset)
				continue
			}

			ts.publishEvent(msgBuf, fo)
			msgBuf.Reset()
			msgBuf.WriteString(init)
			fo.ShiftInt64(line.Offset)
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
