// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package wineventlog

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
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
	emptySpaceScanLength  = 20
	UnknownBytesPerCharacter = 0

	CRITICAL                 = "CRITICAL"
	ERROR                    = "ERROR"
	WARNING                  = "WARNING"
	INFORMATION              = "INFORMATION"
	VERBOSE                  = "VERBOSE"
	UNKNOWN                  = "UNKNOWN"
)

var numbersOfBytesPerCharacter = UnknownBytesPerCharacter

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
	cutOffPeriod := (time.Hour * 24 * 14).Nanoseconds()
	ignoreOlderThanTwoWeeksFilter := fmt.Sprintf(eventIgnoreOldFilter, cutOffPeriod/int64(time.Millisecond))
	if filterLevels != "" {
		filterLevels = "*[System[(" + filterLevels + ") and " + ignoreOlderThanTwoWeeksFilter + "]]"
	} else {
		filterLevels = "*[System[" + ignoreOlderThanTwoWeeksFilter + "]]"
	}

	xml := fmt.Sprintf(eventLogQueryTemplate, path, filterLevels)
	return syscall.UTF16PtrFromString(xml)
}

func UTF16ToUTF8Bytes(in []byte, length uint32) ([]byte, error) {
	var i int
	// Since Windows server 2022, the returned value of used buffer represents for double bytes char count,
	// which is half of the actual buffer used by byte(what older Windows OS returns), checking if the length
	// land on the end of used buffer, if no, double it.
	if numbersOfBytesPerCharacter == UnknownBytesPerCharacter {
		if isTheEndOfContent(in, length) {
			numbersOfBytesPerCharacter = 1
		} else {
			log.Printf("D! Buffer used: %d is returning as double byte character count, doubling it for decoding", length)
			numbersOfBytesPerCharacter = 2
		}
	}

	i = int(length) * numbersOfBytesPerCharacter

	if i%2 != 0 {
		i--
	}

	if i > cap(in) {
		i = cap(in)
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

func isTheEndOfContent(in []byte, length uint32) bool {
	// scan next 20 bytes, if any of them is none '0', return false
	i := int(length)

	if i%2 != 0 {
		i -= 1
	}
	max := len(in)
	if i+emptySpaceScanLength < max {
		max = i+emptySpaceScanLength
	}

	for ; i < max - 2; i += 2 {
		v1 := uint16(in[i+2]) | uint16(in[i+1])<<8
		// Stop at non-null char.
		if v1 != 0 {
			return false
		}
	}
	return true
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
