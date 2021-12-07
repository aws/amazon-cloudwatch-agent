// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// +build windows

package wineventlog

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"encoding/xml"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"golang.org/x/sys/windows"
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

func NewEventLog(name string, levels []string, logGroupName, logStreamName, renderFormat, destination, stateFilePath string, maximumToRead int, retention int) *windowsEventLog {
	eventLog := &windowsEventLog{
		name:          name,
		levels:        levels,
		logGroupName:  logGroupName,
		logStreamName: logStreamName,
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

func (l *windowsEventLog) Init() error {
	go l.runSaveState()
	l.loadState()
	return l.Open()
}

func (l *windowsEventLog) SetOutput(fn func(logs.LogEvent)) {
	if fn == nil {
		return
	}
	l.outputFn = fn
	l.startOnce.Do(func() { go l.run() })
}

func (l *windowsEventLog) Group() string {
	return l.logGroupName
}

func (l *windowsEventLog) Stream() string {
	return l.logStreamName
}

func (l *windowsEventLog) Description() string {
	return fmt.Sprintf("%v%v", l.name, l.levels)
}

func (l *windowsEventLog) Destination() string {
	return l.destination
}

func (l *windowsEventLog) Retention() int {
	return l.retention
}
func (l *windowsEventLog) Stop() {
	close(l.done)
}

func (l *windowsEventLog) run() {
	recordNumber := l.eventOffset
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			records, err := l.read()
			if err == RPC_S_INVALID_BOUND {
				log.Printf("E! [windows_event_log] Due to corrupted/large event, skipping event log with record number %d, and log group name %s", recordNumber, l.logGroupName)
				recordNumber = recordNumber + 1 // Advance to the next event to avoid being stuck
				continue
			}
			if err != nil {
				log.Printf("E! [windows_event_log] Failed to read Windows event logs for log group name %s. Details: %v\n", l.logGroupName, err)
				recordNumber = recordNumber + 1
				continue
			}

			for _, record := range records {
				value, err := record.Value()
				if err != nil {
					log.Printf("E! [windows_event_log] Error happened when collecting windows events : %v", err)
					continue
				}
				recordNumber, _ = strconv.ParseUint(record.System.EventRecordID, 10, 64)
				// TODO: Create and send log event to output fn
				evt := &LogEvent{
					msg:    value,
					t:      record.System.TimeCreated.SystemTime,
					offset: recordNumber,
					src:    l,
				}
				l.outputFn(evt)
			}
		case <-l.done:
			return
		}
	}
}

func (l *windowsEventLog) Open() error {
	bookmark, err := CreateBookmark(l.name, l.eventOffset)
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

	channelPath, err := syscall.UTF16PtrFromString(l.name)
	if err != nil {
		return err
	}

	query, err := CreateQuery(l.name, l.levels)
	if err != nil {
		return err
	}

	// Subscribe for events
	eventHandle, err := EvtSubscribe(0, uintptr(signalEvent), channelPath, query, bookmark, 0, 0, EvtSubscribeStartAfterBookmark)
	if err != nil {
		fmt.Errorf("error when subscribing for events. Details: %v", err)
	}

	l.eventHandle = eventHandle

	return nil
}

func (l *windowsEventLog) Close() error {
	return EvtClose(l.eventHandle)
}

func (l *windowsEventLog) LogGroupName() string {
	return l.logGroupName
}

func (l *windowsEventLog) LogStreamName() string {
	return l.logStreamName
}

func (l *windowsEventLog) EventOffset() uint64 {
	return l.eventOffset
}

func (l *windowsEventLog) SetEventOffset(eventOffset uint64) {
	l.eventOffset = eventOffset
}

func (l *windowsEventLog) Done(offset uint64) {
	l.offsetCh <- offset
}

func (l *windowsEventLog) runSaveState() {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()

	var offset, lastSavedOffset uint64
	for {
		select {
		case o := <-l.offsetCh:

			if o > offset {
				offset = o
			}
		case <-t.C:
			if offset == lastSavedOffset {
				continue
			}
			err := l.saveState(offset)
			if err != nil {
				log.Printf("E! [windows_event_log] Error happened when saving file state %s to file state folder %s: %v", l.logGroupName, l.stateFilePath, err)
				continue
			}
			lastSavedOffset = offset
		case <-l.done:
			err := l.saveState(offset)
			if err != nil {
				log.Printf("E! [windows_event_log] Error happened during final file state saving of logfile %s to file state folder %s, duplicate log maybe sent at next start: %v", l.logGroupName, l.stateFilePath, err)
			}
			break
		}
	}
}

func (l *windowsEventLog) saveState(offset uint64) error {
	if l.stateFilePath == "" || offset == 0 {
		return nil
	}

	content := []byte(strconv.FormatUint(offset, 10) + "\n" + l.logGroupName)
	return ioutil.WriteFile(l.stateFilePath, content, 0644)
}

func (l *windowsEventLog) read() ([]*windowsEventLogRecord, error) {
	maxToRead := l.maxToRead
	var eventHandles []EvtHandle
	defer func() {
		for _, h := range eventHandles {
			EvtClose(h)
		}
	}()

	var numRead uint32
	for {
		eventHandles = make([]EvtHandle, maxToRead)
		err := EvtNext(l.eventHandle, uint32(len(eventHandles)),
			&eventHandles[0], 0, 0, &numRead)

		// Handle special case when events size is too large - retry with smaller size
		if err == RPC_S_INVALID_BOUND {
			if maxToRead == 1 {
				log.Printf("E! [windows_event_log] Out of bounds error due to large events size. Will skip the event as we cannot process it. Details: %v\n", err)
				return nil, err
			}
			log.Printf("W! [windows_event_log] Out of bounds error due to large events size. Retrying with half of the read batch size (%d). Details: %v\n", maxToRead/2, err)
			maxToRead /= 2
			for _, h := range eventHandles {
				EvtClose(h)
			}
			continue
		}

		break
	}
	// Decode the events into objects
	if numRead == 0 {
		return nil, nil
	}

	return l.getRecords(eventHandles[:numRead])
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

func (l *windowsEventLog) getRecords(handles []EvtHandle) (records []*windowsEventLogRecord, err error) {
	//Windows event message supports 31839 characters. https://msdn.microsoft.com/EN-US/library/windows/desktop/aa363679.aspx
	bufferSize := 1 << 17
	for _, evtHandle := range handles {
		renderBuf := make([]byte, bufferSize)
		var outputBuf []byte

		// Notes on the process:
		// - We first call RenderEventXML to get the publisher details. This piece of information is then used
		// for rendering the event and getting a readable XML format that contains the log message.
		// - We can later do more research on comparing other methods to get the publisher details such as EvtCreateRenderContext
		if outputBuf, err = RenderEventXML(evtHandle, renderBuf); err != nil {
			return nil, err
		}

		newRecord := newEventLogRecord(l)
		//we need the "System.TimeCreated.SystemTime"
		xml.Unmarshal(outputBuf, newRecord)
		publisher, _ := syscall.UTF16PtrFromString(newRecord.System.Provider.Name)

		var publisherMetadataEvtHandle EvtHandle
		if publisherMetadataEvtHandle, err = EvtOpenPublisherMetadata(0, publisher, nil, 0, 0); err != nil {
			return nil, err
		}

		var bufferUsed uint32
		if err = EvtFormatMessage(publisherMetadataEvtHandle, evtHandle, 0, 0, 0, EvtFormatMessageXml, uint32(bufferSize), &renderBuf[0], &bufferUsed); err != nil {
			EvtClose(publisherMetadataEvtHandle)
			return nil, err
		}
		EvtClose(publisherMetadataEvtHandle)

		var descriptionBytes []byte
		if descriptionBytes, err = UTF16ToUTF8Bytes(renderBuf, bufferUsed); err != nil {
			return nil, err
		}

		switch l.renderFormat {
		case FormatXml, FormatDefault:
			//XML format
			newRecord.XmlFormatContent = string(descriptionBytes)
		case FormatPlainText:
			//old SSM agent Windows format
			var recordMessage eventMessage
			if err = xml.Unmarshal(descriptionBytes, &recordMessage); err != nil {
				return nil, err
			}

			newRecord.System.Description = recordMessage.Message
		default:
			return nil, fmt.Errorf("format %s is not recognized", l.renderFormat)
		}

		//add record to array
		records = append(records, newRecord)
	}
	return records, err
}

func (l *windowsEventLog) loadState() {
	if _, err := os.Stat(l.stateFilePath); err != nil {
		log.Printf("I! [windows_event_log] The state file for %s does not exist: %v", l.stateFilePath, err)
		return
	}

	byteArray, err := ioutil.ReadFile(l.stateFilePath)
	if err != nil {
		log.Printf("W! [windows_event_log] Issue encountered when reading offset from file %s: %v", l.stateFilePath, err)
		return
	}

	offset, err := strconv.ParseInt(strings.Split(string(byteArray), "\n")[0], 10, 64)
	if err != nil {
		log.Printf("W! [windows_event_log] Issue encountered when parsing offset value %v: %v", byteArray, err)
		return
	}

	log.Printf("I! [windows_event_log] Reading from offset %v in %s", offset, l.stateFilePath)
	l.eventOffset = uint64(offset)
}
