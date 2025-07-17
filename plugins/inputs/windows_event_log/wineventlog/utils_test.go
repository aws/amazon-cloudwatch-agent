// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package wineventlog

import (
	"encoding/hex"
	"syscall"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/state"
)

func TestCreateFilterQuery(t *testing.T) {
	tests := []struct {
		name     string
		levels   []string
		eventIDs []int
		want     string
	}{
		{
			name:     "levels_Test",
			levels:   []string{"Error", "Critical"},
			eventIDs: []int{},
			want:     "*[System[(Level='Error' or Level='Critical') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]",
		},
		{
			name:     "EventID_Test",
			levels:   []string{},
			eventIDs: []int{1001, 1002},
			want:     "*[System[(EventID='1001' or EventID='1002') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]",
		},
		{
			name:     "levels_EventID_Test",
			levels:   []string{"Error", "Critical"},
			eventIDs: []int{4625, 4624},
			want:     "*[System[(Level='Error' or Level='Critical') and (EventID='4625' or EventID='4624') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]",
		},
		{
			name:     "no_Input",
			levels:   []string{},
			eventIDs: []int{},
			want:     "*[System[TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createFilterQuery(tt.levels, tt.eventIDs)
			assert.Equal(t, tt.want, got)

		})
	}
}

func TestCreateQuery(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		levels   []string
		expected string
	}{
		{
			name:     "Single level filter",
			path:     "Application",
			levels:   []string{"2"},
			expected: `<QueryList><Query Id="0"><Select Path="Application">*[System[(Level='2') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
		{
			name:     "Multiple level filters",
			path:     "System",
			levels:   []string{"2", "3", "4"},
			expected: `<QueryList><Query Id="0"><Select Path="System">*[System[(Level='2' or Level='3' or Level='4') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
		{
			name:     "No level filters",
			path:     "Security",
			levels:   []string{},
			expected: `<QueryList><Query Id="0"><Select Path="Security">*[System[TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
		{
			name:     "Empty level filters",
			path:     "Application",
			levels:   nil,
			expected: `<QueryList><Query Id="0"><Select Path="Application">*[System[TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
		{
			name:     "Path with special characters",
			path:     "Microsoft-Windows-Security-Auditing",
			levels:   []string{"2"},
			expected: `<QueryList><Query Id="0"><Select Path="Microsoft-Windows-Security-Auditing">*[System[(Level='2') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000]]]</Select></Query></QueryList>`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ptr, err := CreateQuery(tc.path, tc.levels)
			assert.NoError(t, err)
			assert.NotNil(t, ptr)
			assert.Equal(t, tc.expected, utf16PtrToString(ptr))
		})
	}
}

func TestCreateRangeQuery(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		levels   []string
		r        state.Range
		expected string
	}{
		{
			name:     "Single level with range",
			path:     "Application",
			levels:   []string{"2"},
			r:        state.NewRange(100, 200),
			expected: `<QueryList><Query Id="0"><Select Path="Application">*[System[(Level='2') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000] and EventRecordID &gt; 100 and EventRecordID &lt;= 200]]</Select></Query></QueryList>`,
		},
		{
			name:     "Multiple levels with range",
			path:     "System",
			levels:   []string{"2", "3"},
			r:        state.NewRange(1000, 2000),
			expected: `<QueryList><Query Id="0"><Select Path="System">*[System[(Level='2' or Level='3') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000] and EventRecordID &gt; 1000 and EventRecordID &lt;= 2000]]</Select></Query></QueryList>`,
		},
		{
			name:     "No levels with range",
			path:     "Security",
			levels:   []string{},
			r:        state.NewRange(50, 150),
			expected: `<QueryList><Query Id="0"><Select Path="Security">*[System[TimeCreated[timediff(@SystemTime) &lt;= 1209600000] and EventRecordID &gt; 50 and EventRecordID &lt;= 150]]</Select></Query></QueryList>`,
		},
		{
			name:     "Empty levels with range",
			path:     "Application",
			levels:   nil,
			r:        state.NewRange(0, 100),
			expected: `<QueryList><Query Id="0"><Select Path="Application">*[System[TimeCreated[timediff(@SystemTime) &lt;= 1209600000] and EventRecordID &gt; 0 and EventRecordID &lt;= 100]]</Select></Query></QueryList>`,
		},
		{
			name:     "Large range values",
			path:     "System",
			levels:   []string{"2", "3", "4"},
			r:        state.NewRange(999999, 1000000),
			expected: `<QueryList><Query Id="0"><Select Path="System">*[System[(Level='2' or Level='3' or Level='4') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000] and EventRecordID &gt; 999999 and EventRecordID &lt;= 1000000]]</Select></Query></QueryList>`,
		},
		{
			name:     "Zero start range",
			path:     "Test",
			levels:   []string{"2"},
			r:        state.NewRange(0, 1),
			expected: `<QueryList><Query Id="0"><Select Path="Test">*[System[(Level='2') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000] and EventRecordID &gt; 0 and EventRecordID &lt;= 1]]</Select></Query></QueryList>`,
		},
		{
			name:     "Path with special characters and range",
			path:     "Microsoft-Windows-Kernel-General",
			levels:   []string{"2"},
			r:        state.NewRange(12345, 67890),
			expected: `<QueryList><Query Id="0"><Select Path="Microsoft-Windows-Kernel-General">*[System[(Level='2') and TimeCreated[timediff(@SystemTime) &lt;= 1209600000] and EventRecordID &gt; 12345 and EventRecordID &lt;= 67890]]</Select></Query></QueryList>`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ptr, err := CreateRangeQuery(tc.path, tc.levels, tc.r)
			assert.NoError(t, err)
			assert.NotNil(t, ptr)
			assert.Equal(t, tc.expected, utf16PtrToString(ptr))
		})
	}
}

func resetState() {
	NumberOfBytesPerCharacter = 0
}

func utf16PtrToString(ptr *uint16) string {
	utf16Slice := make([]uint16, 0, 1024)
	for i := 0; ; i++ {
		// Get the value at memory address ptr + (i * sizeof(uint16))
		element := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) + uintptr(i)*unsafe.Sizeof(uint16(0))))

		if element == 0 {
			break // Null terminator found
		}
		utf16Slice = append(utf16Slice, element)
	}
	return syscall.UTF16ToString(utf16Slice)
}
