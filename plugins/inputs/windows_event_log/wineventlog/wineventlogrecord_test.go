// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows
// +build windows

package wineventlog

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshalWinEvtRecord(t *testing.T) {
	tests := []struct {
		xml        string
		wEvtRecord windowsEventLogRecord
	}{
		{
			xml: `
<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'>
    <EventData>
        <Data Name='param1'>2022-10-28T22:33:25Z</Data>
        <Data Name='param2'>RulesEngine</Data>
        <Data Name='param3'>2</Data>
    </EventData>
</Event>
			`,
			wEvtRecord: windowsEventLogRecord{
				EventData: EventData{
					Data: []Datum{
						{"2022-10-28T22:33:25Z"},
						{"RulesEngine"},
						{"2"},
					},
				},
			},
		},
		{
			xml: `
<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'>
    <UserData>
        <RmSessionEvent xmlns='http://www.microsoft.com/2005/08/Windows/Reliability/RestartManager/'>
            <RmSessionId>0</RmSessionId>
            <UTCStartTime>2022-10-26T20:24:13.4253261Z</UTCStartTime>
        </RmSessionEvent>
    </UserData>
</Event>
			`,
			wEvtRecord: windowsEventLogRecord{
				UserData: UserData{
					Data: []Datum{
						{"0"},
						{"2022-10-26T20:24:13.4253261Z"},
					},
				},
			},
		},
	}

	for _, test := range tests {
		var record windowsEventLogRecord
		xml.Unmarshal([]byte(test.xml), &record)
		assert.Equal(t, test.wEvtRecord, record)
	}
}
