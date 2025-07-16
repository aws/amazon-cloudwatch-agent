// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package wineventlog

import (
	"encoding/xml"
	"fmt"
	"log"
	"strconv"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

// https://msdn.microsoft.com/en-us/library/windows/desktop/aa385588(v=vs.85).aspx
// https://msdn.microsoft.com/en-us/library/windows/desktop/ms681382(v=vs.85).aspx
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa385525(v=vs.85).aspx
const (
	RPC_S_INVALID_BOUND syscall.Errno = 1734

	collectionInterval  = time.Second
	saveStateInterval   = 100 * time.Millisecond
	subscribeMaxRetries = 3

	apiEvtSubscribe = "EvtSubscribe"
	apiEvtQuery     = "EvtQuery"
	apiEvtClose     = "EvtClose"
)

var winEventAPI = NewWindowsEventAPI()

type wevtAPIError struct {
	api  string
	name string
	err  error
}

func (e *wevtAPIError) Error() string {
	return fmt.Sprintf("%s(), name %s, err %v", e.api, e.name, e.err)
}

type windowsEventLog struct {
	name          string
	levels        []string
	logGroupName  string
	logStreamName string
	logGroupClass string
	renderFormat  string
	maxToRead     int // Maximum number returned in one read.
	destination   string
	stateManager  state.FileRangeManager

	eventHandle   EvtHandle
	eventOffset   uint64
	gapsToRead    state.RangeList
	retention     int
	outputFn      func(logs.LogEvent)
	done          chan struct{}
	startOnce     sync.Once
	resubscribeCh chan struct{}
}

func NewEventLog(name string, levels []string, logGroupName, logStreamName, renderFormat, destination string, stateManager state.FileRangeManager, maximumToRead int, retention int, logGroupClass string) *windowsEventLog {
	eventLog := &windowsEventLog{
		name:          name,
		levels:        levels,
		logGroupName:  logGroupName,
		logStreamName: logStreamName,
		logGroupClass: logGroupClass,
		renderFormat:  renderFormat,
		maxToRead:     maximumToRead,
		destination:   destination,
		stateManager:  stateManager,
		retention:     retention,

		gapsToRead: nil,

		done:          make(chan struct{}),
		resubscribeCh: make(chan struct{}),
	}
	return eventLog
}

func (w *windowsEventLog) Init() error {
	go w.stateManager.Run(state.Notification{Done: w.done})
	restored, _ := w.stateManager.Restore()
	// Do note that the end offset is inclusive here as opposed to exclusive like done
	// in logfile. This is because we use the EvtSubscribeStartAfterBookmark flag in
	// EvtSubscribe.
	w.eventOffset = restored.Last().EndOffset()
	if !restored.OnlyUseMaxOffset() {
		w.gapsToRead = state.InvertRanges(restored)
	}
	return w.Open()
}

func (w *windowsEventLog) SetOutput(fn func(logs.LogEvent)) {
	if fn == nil {
		return
	}
	w.outputFn = fn
	w.startOnce.Do(func() { go w.run() })
}

func (w *windowsEventLog) Group() string {
	return w.logGroupName
}

func (w *windowsEventLog) Stream() string {
	return w.logStreamName
}

func (w *windowsEventLog) Description() string {
	return fmt.Sprintf("%v%v", w.name, w.levels)
}

func (w *windowsEventLog) Destination() string {
	return w.destination
}

func (w *windowsEventLog) Retention() int {
	return w.retention
}

func (w *windowsEventLog) Class() string {
	return w.logGroupClass
}

func (w *windowsEventLog) Stop() {
	close(w.done)
}

func (w *windowsEventLog) Entity() *cloudwatchlogs.Entity {
	return nil
}

func (w *windowsEventLog) run() {
	ticker := time.NewTicker(collectionInterval)
	defer ticker.Stop()

	r := state.Range{}
	retryCount := 0
	var shouldResubscribe bool
	for {
		select {
		case <-w.resubscribeCh:
			shouldResubscribe = true
		case <-ticker.C:
			if shouldResubscribe {
				restored, _ := w.stateManager.Restore()
				w.eventOffset = restored.Last().EndOffset()
				if !restored.OnlyUseMaxOffset() {
					w.gapsToRead = state.InvertRanges(restored)
				}
				if err := w.resubscribe(); err != nil {
					log.Printf("E! [wineventlog] Unable to re-subscribe: %v", err)
					retryCount++
					if retryCount >= subscribeMaxRetries {
						log.Printf("D! [wineventlog] Max subscribe retries reached: %d", subscribeMaxRetries)
						shouldResubscribe = false
						retryCount = 0
					}
				} else {
					log.Printf("D! [wineventlog] Re-subscribed to %s", w.name)
					shouldResubscribe = false
				}
			}
			// Prioritize gaps to read on this tick of the timer
			var records []*windowsEventLogRecord
			if len(w.gapsToRead) > 0 {
				records = w.readGaps()
			} else {
				records = w.read()
			}
			for _, record := range records {
				value, err := record.Value()
				if err != nil {
					log.Printf("E! [wineventlog] Error happened when collecting windows events: %v", err)
					continue
				}
				recordNumber, _ := strconv.ParseUint(record.System.EventRecordID, 10, 64)
				r.Shift(recordNumber)
				evt := &LogEvent{
					msg:    value,
					t:      record.System.TimeCreated.SystemTime,
					offset: r,
					src:    w,
				}
				w.outputFn(evt)
			}
		case <-w.done:
			return
		}
	}
}

// Open subscription for events. Instead of failing the subscription if the eventlog name has not been registered,
// log the error.
func (w *windowsEventLog) Open() error {
	err := w.open()
	if werr, ok := err.(*wevtAPIError); ok && werr.api == apiEvtSubscribe {
		log.Printf("W! [wineventlog] %v", err)
		return nil
	}
	return err
}

func (w *windowsEventLog) open() error {
	bookmark, err := CreateBookmark(winEventAPI, w.name, w.eventOffset)
	if err != nil {
		return err
	}
	defer winEventAPI.EvtClose(bookmark)
	// Using a pull subscription to receive events. See:
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa385771(v=vs.85).aspx#pull
	signalEvent, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return nil
	}
	channelPath, err := syscall.UTF16PtrFromString(w.name)
	if err != nil {
		return err
	}
	query, err := CreateQuery(w.name, w.levels)
	if err != nil {
		return err
	}
	eventHandle, err := winEventAPI.EvtSubscribe(0, uintptr(signalEvent), channelPath, query, bookmark, 0, 0, EvtSubscribeStartAfterBookmark)
	if err != nil {
		return &wevtAPIError{api: apiEvtSubscribe, name: w.name, err: err}
	}
	w.eventHandle = eventHandle
	return nil
}

func (w *windowsEventLog) openAtRange(r state.Range) (EvtHandle, error) {
	channelPath, err := syscall.UTF16PtrFromString(w.name)
	if err != nil {
		return 0, err
	}
	query, err := CreateRangeQuery(w.name, w.levels, r)
	if err != nil {
		return 0, err
	}
	eventHandle, err := winEventAPI.EvtQuery(0, channelPath, query, EvtQueryChannelPath)
	if err != nil {
		return 0, &wevtAPIError{api: apiEvtQuery, name: w.name, err: err}
	}
	return eventHandle, nil
}

func (w *windowsEventLog) Close() error {
	return winEventAPI.EvtClose(w.eventHandle)
}

// resubscribe closes the event subscription based on the event handle and resets the handle to the
// same state as an EvtSubscribe failure (0) before attempting to open another event subscription.
func (w *windowsEventLog) resubscribe() error {
	if w.eventHandle != 0 {
		if err := w.Close(); err != nil {
			return &wevtAPIError{api: apiEvtClose, name: w.name, err: err}
		}
	}
	w.eventHandle = EvtHandle(0)
	return w.open()
}

func (w *windowsEventLog) LogGroupName() string {
	return w.logGroupName
}

func (w *windowsEventLog) LogStreamName() string {
	return w.logStreamName
}

func (w *windowsEventLog) EventOffset() uint64 {
	return w.eventOffset
}

func (w *windowsEventLog) SetEventOffset(eventOffset uint64) {
	w.eventOffset = eventOffset
}

func (w *windowsEventLog) ResubscribeCh() chan struct{} {
	return w.resubscribeCh
}

func (w *windowsEventLog) readGaps() []*windowsEventLogRecord {
	var records []*windowsEventLogRecord
	for _, r := range w.gapsToRead {
		if r.IsEndOffsetUnbounded() {
			continue
		}

		readRecords, err := w.readGap(r)
		if err != nil {
			continue
		}
		records = append(records, readRecords...)
	}

	// Clear out processed gaps
	w.gapsToRead = nil

	return records
}

func (w *windowsEventLog) readGap(r state.Range) ([]*windowsEventLogRecord, error) {
	handle, err := w.openAtRange(r)
	defer func() {
		winEventAPI.EvtClose(handle)
	}()
	if err != nil {
		return nil, err
	}
	readRecords := w.readFromHandle(handle)
	return readRecords, nil
}

func (w *windowsEventLog) read() []*windowsEventLogRecord {
	return w.readFromHandle(w.eventHandle)
}

// readFromHandle reads events from a specific event handle (used for gap reading)
func (w *windowsEventLog) readFromHandle(eventHandle EvtHandle) []*windowsEventLogRecord {
	maxToRead := w.maxToRead
	var eventHandles []EvtHandle
	defer func() {
		for _, h := range eventHandles {
			winEventAPI.EvtClose(h)
		}
	}()

	var numRead uint32
	for {
		eventHandles = make([]EvtHandle, maxToRead)
		err := winEventAPI.EvtNext(eventHandle, uint32(len(eventHandles)),
			&eventHandles[0], 0, 0, &numRead)
		// Handle special case when events size is too large - retry with smaller size
		if err == RPC_S_INVALID_BOUND {
			if maxToRead == 1 {
				log.Printf("E! [wineventlog] Out of bounds error due to large events size. Will skip the event as we cannot process it. Details: %v\n", err)
				return nil
			}
			log.Printf("W! [wineventlog] Out of bounds error due to large events size. Retrying with half of the read batch size (%d). Details: %v\n", maxToRead/2, err)
			maxToRead /= 2
			for _, h := range eventHandles {
				winEventAPI.EvtClose(h)
			}
			continue
		}
		break
	}
	// Decode the events into objects
	return w.getRecords(eventHandles[:numRead])
}

type LogEvent struct {
	msg    string
	t      time.Time
	offset state.Range
	src    *windowsEventLog
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

// getRecords attempts to render and format each of the given EvtHandles.
// If one handle has an error, continue on because something is better than nothing.
func (w *windowsEventLog) getRecords(handles []EvtHandle) (records []*windowsEventLogRecord) {
	for _, evtHandle := range handles {
		r, err := w.getRecord(evtHandle)
		if err == nil {
			records = append(records, r)
		} else {
			log.Printf("I! [wineventlog] %v", err)
		}
	}
	return records
}

// getRecord attemps to render and format the message for the given EvtHandle.
func (w *windowsEventLog) getRecord(evtHandle EvtHandle) (*windowsEventLogRecord, error) {
	// Notes on the process:
	// - We first call RenderEventXML to get the publisher details. This piece of information is then used
	// for rendering the event and getting a readable XML format that contains the log message.
	// - We can later do more research on comparing other methods to get the publisher details such as EvtCreateRenderContext

	// Windows event message supports 31839 characters. https://msdn.microsoft.com/EN-US/library/windows/desktop/aa363679.aspx
	bufferSize := 1 << 17
	renderBuf := make([]byte, bufferSize)
	outputBuf, err := RenderEventXML(winEventAPI, evtHandle, renderBuf)
	if err != nil {
		return nil, fmt.Errorf("RenderEventXML() err %v", err)
	}
	newRecord := newEventLogRecord(w)
	//we need the "System.TimeCreated.SystemTime"
	xml.Unmarshal(outputBuf, newRecord)
	publisher, _ := syscall.UTF16PtrFromString(newRecord.System.Provider.Name)
	publisherMetadataEvtHandle, err := winEventAPI.EvtOpenPublisherMetadata(0, publisher, nil, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("EvtOpenPublisherMetadata() publisher %v, err %v", newRecord.System.Provider.Name, err)
	}
	var bufferUsed uint32
	err = winEventAPI.EvtFormatMessage(publisherMetadataEvtHandle, evtHandle, 0, 0, 0, EvtFormatMessageXml, uint32(bufferSize), &renderBuf[0], &bufferUsed)
	winEventAPI.EvtClose(publisherMetadataEvtHandle)
	if err != nil && bufferUsed == 0 {
		return nil, fmt.Errorf("EvtFormatMessage() publisher %v, err %v", newRecord.System.Provider.Name, err)
	}
	descriptionBytes, err := UTF16ToUTF8BytesForWindowsEventBuffer(renderBuf, bufferUsed)
	if err != nil {
		return nil, fmt.Errorf("utf16ToUTF8Bytes() err %v", err)
	}

	// The insertion strings could be in either EventData or UserData
	// Notes on the insertion strings:
	// - The EvtFormatMessage has the valueCount and values parameters, yet it does not work when we tried passing
	//   EventData/UserData into those parameters. We can later do more research on making EvtFormatMessage with
	//   valueCount and values parameters works and compare if there is any benefit.
	dataValues := newRecord.EventData.Data
	// The UserData section is used if EventData is empty
	if len(dataValues) == 0 {
		dataValues = newRecord.UserData.Data
	}
	switch w.renderFormat {
	case FormatXml, FormatDefault:
		//XML format
		newRecord.XmlFormatContent = insertPlaceholderValues(string(descriptionBytes), dataValues)
	case FormatPlainText:
		//old SSM agent Windows format
		var recordMessage eventMessage
		err = xml.Unmarshal(descriptionBytes, &recordMessage)
		if err != nil {
			return nil, fmt.Errorf("Unmarshal() err %v", err)
		}
		newRecord.System.Description = insertPlaceholderValues(recordMessage.Message, dataValues)
	default:
		return nil, fmt.Errorf("renderFormat is not recognized, %s", w.renderFormat)
	}
	return newRecord, nil
}
