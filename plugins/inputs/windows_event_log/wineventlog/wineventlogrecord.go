// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package wineventlog

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"time"
)

const (
	FormatXml       = "xml"
	FormatPlainText = "text"
	FormatDefault   = ""
)

type eventMessage struct {
	Message string `xml:"RenderingInfo>Message"`
}

// For Windows versions later than 2003
type windowsEventLogRecord struct {
	windowsEventLog *windowsEventLog

	XmlFormatContent string

	System struct {
		Description   string
		Level         string `xml:"Level"`
		EventRecordID string `xml:"EventRecordID"`

		TimeCreated struct {
			SystemTime time.Time `xml:"SystemTime,attr"`
		} `xml:"TimeCreated"`
		Channel         string `xml:"Channel"`
		Computer        string `xml:"Computer"`
		EventIdentifier struct {
			ID string `xml:",chardata"`
		} `xml:"EventID"`
		Provider struct {
			Name string `xml:"Name,attr"`
		} `xml:"Provider"`
	} `xml:"System"`

	EventData EventData `xml:"EventData"`
	UserData  UserData  `xml:"UserData"`
}

func newEventLogRecord(l *windowsEventLog) *windowsEventLogRecord {
	record := new(windowsEventLogRecord)
	record.windowsEventLog = l
	return record
}

func (record *windowsEventLogRecord) RecordId() string {
	return record.System.EventRecordID
}

func (record *windowsEventLogRecord) Value() (valueString string, err error) {
	switch record.windowsEventLog.renderFormat {
	case FormatXml, FormatDefault:
		//XML format
		valueString = record.XmlFormatContent
	case FormatPlainText:
		//old SSM agent Windows format
		levelId, _ := strconv.ParseInt(record.System.Level, 10, 32)
		valueString = fmt.Sprintf("[%s] [%s] [%s] [%s] [%s] [%s]", record.System.Channel,
			WindowsEventLogLevelName(int32(levelId)), record.System.EventIdentifier.ID, record.System.Provider.Name,
			record.System.Computer, record.System.Description)
	default:
		err = fmt.Errorf("renderFormat %s is not recognized", record.windowsEventLog.renderFormat)
	}

	return valueString, err
}

func (record *windowsEventLogRecord) Timestamp() string {
	return fmt.Sprint(record.System.TimeCreated.SystemTime.UnixNano())
}

type Datum struct {
	Value string `xml:",chardata"`
}

type EventData struct {
	Data []Datum `xml:",any"`
}

type UserData struct {
	Data []Datum `xml:",any"`
}

// UnmarshalXML unmarshals the UserData section in the windows event xml to UserData struct
//
// UserData has slightly different schema than EventData so that we need to override this
// to get similar structure
// https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-userdatatype-complextype
// https://learn.microsoft.com/en-us/windows/win32/wes/eventschema-eventdatatype-complextype
func (u *UserData) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	in := EventData{}

	// Read tokens until we find the first StartElement then unmarshal it.
	for {
		t, err := d.Token()
		if err != nil {
			return err
		}

		if se, ok := t.(xml.StartElement); ok {
			err = d.DecodeElement(&in, &se)
			if err != nil {
				return err
			}

			u.Data = in.Data
			d.Skip()
			break
		}
	}

	return nil
}
