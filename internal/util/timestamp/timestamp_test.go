// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package timestamp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRegexWithNamedCaptureGroup(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		expected string
	}{
		{
			name:     "simple date time",
			format:   "%Y-%m-%d %H:%M:%S",
			expected: `(?P<timestamp>\d{4}-\s{0,1}\d{1,2}-\s{0,1}\d{1,2} \d{2}:\d{2}:\d{2})`,
		},
		{
			name:     "date with milliseconds",
			format:   "%Y-%m-%d %H:%M:%S.%f",
			expected: `(?P<timestamp>\d{4}-\s{0,1}\d{1,2}-\s{0,1}\d{1,2} \d{2}:\d{2}:\d{2}\.(\d{1,9}))`,
		},
		{
			name:     "syslog format",
			format:   "%b %d %H:%M:%S",
			expected: `(?P<timestamp>\w{3} \s{0,1}\d{1,2} \d{2}:\d{2}:\d{2})`,
		},
		{
			name:     "with timezone",
			format:   "%Y-%m-%d %H:%M:%S %Z",
			expected: `(?P<timestamp>\d{4}-\s{0,1}\d{1,2}-\s{0,1}\d{1,2} \d{2}:\d{2}:\d{2} \w{3})`,
		},
		{
			name:     "12 hour format",
			format:   "%Y-%m-%d %I:%M:%S %p",
			expected: `(?P<timestamp>\d{4}-\s{0,1}\d{1,2}-\s{0,1}\d{1,2} \d{2}:\d{2}:\d{2} \w{2})`,
		},
		{
			name:     "format starting with month",
			format:   "%-m/%-d/%Y %H:%M:%S",
			expected: `(?P<timestamp>\d{1,2}/\s{0,1}\d{1,2}/\d{4} \d{2}:\d{2}:\d{2})`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildRegexWithNamedCaptureGroup(tt.format)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildRegex(t *testing.T) {
	result := BuildRegex("%Y-%m-%d %H:%M:%S")
	assert.Equal(t, `\d{4}-\s{0,1}\d{1,2}-\s{0,1}\d{1,2} \d{2}:\d{2}:\d{2}`, result)
}

func TestBuildRegex_EscapesSpecialChars(t *testing.T) {
	result := BuildRegex("%Y.%m.%d")
	assert.Equal(t, `\d{4}\.\s{0,1}\d{1,2}\.\s{0,1}\d{1,2}`, result)
}

func TestBuildLayout(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		expected string
	}{
		{
			name:     "simple date time",
			format:   "%Y-%m-%d %H:%M:%S",
			expected: "2006-1-_2 15:04:05",
		},
		{
			name:     "non-padded month and day",
			format:   "%-m/%-d/%Y %H:%M:%S",
			expected: "1/_2/2006 15:04:05",
		},
		{
			name:     "with fractional seconds (dot in format)",
			format:   "%Y-%m-%d %H:%M:%S.%f",
			expected: "2006-1-_2 15:04:05.999999999",
		},
		{
			name:     "with fractional seconds (no dot in format)",
			format:   "%Y-%m-%d %H:%M:%S%f",
			expected: "2006-1-_2 15:04:05.999999999",
		},
		{
			name:     "12 hour format",
			format:   "%Y-%m-%d %I:%M:%S %p",
			expected: "2006-1-_2 03:04:05 PM",
		},
		{
			name:     "ISO8601 with T",
			format:   "%Y-%m-%dT%H:%M:%S",
			expected: "2006-1-_2T15:04:05",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildLayout(tt.format)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildLayout_ParsesTimestamps(t *testing.T) {
	tests := []struct {
		name   string
		format string
		inputs []string
		want   time.Time
	}{
		{
			name:   "padded month parses",
			format: "%Y-%m-%d %H:%M:%S",
			inputs: []string{"2024-01-02 07:10:06", "2024-1-02 07:10:06"},
			want:   time.Date(2024, 1, 2, 7, 10, 6, 0, time.UTC),
		},
		{
			name:   "non-padded directives",
			format: "%-m/%-d/%Y %H:%M:%S",
			inputs: []string{"1/2/2024 07:10:06", "01/02/2024 07:10:06"},
			want:   time.Date(2024, 1, 2, 7, 10, 6, 0, time.UTC),
		},
		{
			name:   "ISO8601",
			format: "%Y-%m-%dT%H:%M:%S",
			inputs: []string{"2024-12-31T23:59:59"},
			want:   time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
		},
		{
			name:   "fractional seconds 3 digits",
			format: "%Y-%m-%d %H:%M:%S.%f",
			inputs: []string{"2024-01-02 07:10:06.123"},
			want:   time.Date(2024, 1, 2, 7, 10, 6, 123000000, time.UTC),
		},
		{
			name:   "fractional seconds 6 digits",
			format: "%Y-%m-%d %H:%M:%S.%f",
			inputs: []string{"2024-01-02 07:10:06.123456"},
			want:   time.Date(2024, 1, 2, 7, 10, 6, 123456000, time.UTC),
		},
		{
			name:   "fractional seconds 9 digits",
			format: "%Y-%m-%d %H:%M:%S.%f",
			inputs: []string{"2024-01-02 07:10:06.123456789"},
			want:   time.Date(2024, 1, 2, 7, 10, 6, 123456789, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := BuildLayout(tt.format)
			for _, input := range tt.inputs {
				parsed, err := time.Parse(layout, input)
				require.NoError(t, err, "layout=%q input=%q", layout, input)
				assert.Equal(t, tt.want, parsed)
			}
		})
	}
}
