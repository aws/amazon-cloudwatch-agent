// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package wineventlog

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"syscall"
	"time"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const (
	bookmarkTemplate      = `<BookmarkList><Bookmark Channel="%s" RecordId="%d" IsCurrent="True"/></BookmarkList>`
	eventLogQueryTemplate = `<QueryList><Query Id="0"><Select Path="%s">%s</Select></Query></QueryList>`
	eventLogLevelFilter   = "Level='%s'"
	eventIgnoreOldFilter  = "TimeCreated[timediff(@SystemTime) &lt;= %d]"

	CRITICAL    = "CRITICAL"
	ERROR       = "ERROR"
	WARNING     = "WARNING"
	INFORMATION = "INFORMATION"
	VERBOSE     = "VERBOSE"
	UNKNOWN     = "UNKNOWN"
)

func RenderEventXML(eventHandle EvtHandle, renderBuf []byte) ([]byte, error) {
	var bufferUsed, propertyCount uint32

	if err := EvtRender(0, eventHandle, EvtRenderEventXml, uint32(len(renderBuf)), &renderBuf[0], &bufferUsed, &propertyCount); err != nil {
		return nil, fmt.Errorf("error when rendering events. Details: %v", err)
	}

	return UTF16ToUTF8Bytes(renderBuf, bufferUsed)
}

func CreateBookmark(channel string, recordID uint64) (h EvtHandle, err error) {
	xml := fmt.Sprintf(bookmarkTemplate, channel, recordID)
	p, err := syscall.UTF16PtrFromString(xml)
	if err != nil {
		return 0, err
	}
	h, err = EvtCreateBookmark(p)
	if err != nil {
		return 0, fmt.Errorf("error when creating a bookmark. Details: %v", err)
	}
	return h, nil
}

func CreateQuery(path string, levels []string) (*uint16, error) {
	var filterLevels string
	for _, level := range levels {
		if filterLevels == "" {
			filterLevels = fmt.Sprintf(eventLogLevelFilter, level)
		} else {
			filterLevels = filterLevels + " or " + fmt.Sprintf(eventLogLevelFilter, level)
		}
	}

	//Ignore events older than 2 weeks
	cutoOffPeriod := (time.Hour * 24 * 14).Nanoseconds()
	ignoreOlderThanTwoWeeksFilter := fmt.Sprintf(eventIgnoreOldFilter, cutoOffPeriod/int64(time.Millisecond))
	if filterLevels != "" {
		filterLevels = "*[System[(" + filterLevels + ") and " + ignoreOlderThanTwoWeeksFilter + "]]"
	} else {
		filterLevels = "*[System[" + ignoreOlderThanTwoWeeksFilter + "]]"
	}

	xml := fmt.Sprintf(eventLogQueryTemplate, path, filterLevels)
	return syscall.UTF16PtrFromString(xml)
}

func UTF16ToUTF8Bytes(in []byte, length uint32) ([]byte, error) {
	i := length

	if length%2 != 0 {
		i = length - 1
	}

	for ; i-2 > 0; i -= 2 {
		v1 := uint16(in[i-2]) | uint16(in[i-1])<<8
		// Stop at non-null char.
		if v1 != 0 {
			break
		}
	}

	win16be := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	utf16bom := unicode.BOMOverride(win16be.NewDecoder())
	unicodeReader := transform.NewReader(bytes.NewReader(in[:i]), utf16bom)
	decoded, err := ioutil.ReadAll(unicodeReader)
	return decoded, err
}

func WindowsEventLogLevelName(levelId int32) string {
	switch levelId {
	case 1:
		return CRITICAL
	case 2:
		return ERROR
	case 3:
		return WARNING
	case 0, 4:
		return INFORMATION
	case 5:
		return VERBOSE
	default:
		return UNKNOWN
	}
}
