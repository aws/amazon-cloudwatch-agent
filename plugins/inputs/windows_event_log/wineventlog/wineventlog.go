// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package wineventlog

import (
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/aws/amazon-cloudwatch-agent/logs"
)

// https://msdn.microsoft.com/en-us/library/windows/desktop/aa385588(v=vs.85).aspx
// https://msdn.microsoft.com/en-us/library/windows/desktop/ms681382(v=vs.85).aspx
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa385525(v=vs.85).aspx
const (
	RPC_S_INVALID_BOUND syscall.Errno = 1734
)

type windowsEventLog struct {
	name          string
	levels        []string
	logGroupName  string
	logStreamName string
	logGroupClass string
	renderFormat  string
	maxToRead     int // Maximum number returned in one read.
	destination   string
	stateFilePath string

	eventHandle EvtHandle
	eventOffset uint64
	retention   int
	outputFn    func(logs.LogEvent)
	offsetCh    chan uint64
	done        chan struct{}
	startOnce   sync.Once
}

func NewEventLog(name string, levels []string, logGroupName, logStreamName, renderFormat, destination, stateFilePath string, maximumToRead int, retention int, logGroupClass string) *windowsEventLog {
	eventLog := &windowsEventLog{
		name:          name,
		levels:        levels,
		logGroupName:  logGroupName,
		logStreamName: logStreamName,
		logGroupClass: logGroupClass,
		renderFormat:  renderFormat,
		maxToRead:     maximumToRead,
		destination:   destination,
		stateFilePath: stateFilePath,
		retention:     retention,

		offsetCh: make(chan uint64, 100),
		done:     make(chan struct{}),
	}
	return eventLog
}

func (w *windowsEventLog) Init() error {
	go w.runSaveState()
	w.loadState()
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

func (w *windowsEventLog) run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			records := w.read()
			for _, record := range records {
				value, err := record.Value()
				if err != nil {
					log.Printf("E! [wineventlog] Error happened when collecting windows events : %v", err)
					continue
				}
				recordNumber, _ := strconv.ParseUint(record.System.EventRecordID, 10, 64)
				evt := &LogEvent{
					msg:    value,
					t:      record.System.TimeCreated.SystemTime,
					offset: recordNumber,
					src:    w,
				}
				w.outputFn(evt)
			}
		case <-w.done:
			return
		}
	}
}

func (w *windowsEventLog) Open() error {
	bookmark, err := CreateBookmark(w.name, w.eventOffset)
	if err != nil {
		return err
	}
	defer EvtClose(bookmark)
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
	// Subscribe for events.
	// This will fail if the eventlog name has not been registered.
	// However returning an error would mean the the plugin won't monitor other
	// eventlogs.
	eventHandle, err := EvtSubscribe(0, uintptr(signalEvent), channelPath, query, bookmark, 0, 0, EvtSubscribeStartAfterBookmark)
	if err != nil {
		log.Printf("W! [wineventlog] EvtSubscribe(), name %v, err %v", w.name, err)
	}
	w.eventHandle = eventHandle
	return nil
}

func (w *windowsEventLog) Close() error {
	return EvtClose(w.eventHandle)
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

func (w *windowsEventLog) Done(offset uint64) {
	w.offsetCh <- offset
}

func (w *windowsEventLog) runSaveState() {
	t := time.NewTicker(100 * time.Millisecond)
	defer w.Stop()

	var offset, lastSavedOffset uint64
	for {
		select {
		case o := <-w.offsetCh:

			if o > offset {
				offset = o
			}
		case <-t.C:
			if offset == lastSavedOffset {
				continue
			}
			err := w.saveState(offset)
			if err != nil {
				log.Printf("E! [wineventlog] Error happened when saving file state %s to file state folder %s: %v", w.logGroupName, w.stateFilePath, err)
				continue
			}
			lastSavedOffset = offset
		case <-w.done:
			err := w.saveState(offset)
			if err != nil {
				log.Printf("E! [wineventlog] Error happened during final file state saving of logfile %s to file state folder %s, duplicate log maybe sent at next start: %v", w.logGroupName, w.stateFilePath, err)
			}
			break
		}
	}
}

func (w *windowsEventLog) saveState(offset uint64) error {
	if w.stateFilePath == "" || offset == 0 {
		return nil
	}

	content := []byte(strconv.FormatUint(offset, 10) + "\n" + w.logGroupName)
	return os.WriteFile(w.stateFilePath, content, 0644)
}

func (w *windowsEventLog) read() []*windowsEventLogRecord {
	maxToRead := w.maxToRead
	var eventHandles []EvtHandle
	defer func() {
		for _, h := range eventHandles {
			EvtClose(h)
		}
	}()

	var numRead uint32
	for {
		eventHandles = make([]EvtHandle, maxToRead)
		err := EvtNext(w.eventHandle, uint32(len(eventHandles)),
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
				EvtClose(h)
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
	offset uint64
	src    *windowsEventLog
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
	outputBuf, err := RenderEventXML(evtHandle, renderBuf)
	if err != nil {
		return nil, fmt.Errorf("RenderEventXML() err %v", err)
	}
	newRecord := newEventLogRecord(w)
	//we need the "System.TimeCreated.SystemTime"
	xml.Unmarshal(outputBuf, newRecord)
	publisher, _ := syscall.UTF16PtrFromString(newRecord.System.Provider.Name)
	publisherMetadataEvtHandle, err := EvtOpenPublisherMetadata(0, publisher, nil, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("EvtOpenPublisherMetadata() publisher %v, err %v", newRecord.System.Provider.Name, err)
	}
	var bufferUsed uint32
	err = EvtFormatMessage(publisherMetadataEvtHandle, evtHandle, 0, 0, 0, EvtFormatMessageXml, uint32(bufferSize), &renderBuf[0], &bufferUsed)
	EvtClose(publisherMetadataEvtHandle)
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

func (w *windowsEventLog) loadState() {
	if _, err := os.Stat(w.stateFilePath); err != nil {
		log.Printf("I! [wineventlog] The state file for %s does not exist: %v", w.stateFilePath, err)
		return
	}
	byteArray, err := os.ReadFile(w.stateFilePath)
	if err != nil {
		log.Printf("W! [wineventlog] Issue encountered when reading offset from file %s: %v", w.stateFilePath, err)
		return
	}
	offset, err := strconv.ParseInt(strings.Split(string(byteArray), "\n")[0], 10, 64)
	if err != nil {
		log.Printf("W! [wineventlog] Issue encountered when parsing offset value %v: %v", byteArray, err)
		return
	}
	log.Printf("I! [wineventlog] Reading from offset %v in %s", offset, w.stateFilePath)
	w.eventOffset = uint64(offset)
}
