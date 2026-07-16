// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package timestamp

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
