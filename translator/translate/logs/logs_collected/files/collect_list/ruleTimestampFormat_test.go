// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimestampRegexRule(t *testing.T) {
	regex := new(TimestampRegex)
	type want struct {
		key   string
		value interface{}
	}
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *want
		wantErr error
	}{
		"WithNonZeroPaddedOptions": {
			input: map[string]interface{}{
				"timestamp_format": "%-m %-d %H:%M:%S",
			},
			want: &want{
				key:   "timestamp_regex",
				value: "(\\d{1,2} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})",
			},
		},
		"WithZeroPaddedOptions": {
			input: map[string]interface{}{
				"timestamp_format": "%m %d %H:%M:%S",
			},
			want: &want{
				key:   "timestamp_regex",
				value: "(\\d{1,2} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})",
			},
		},
		"WithZeroPaddedMonthWord": {
			input: map[string]interface{}{
				"timestamp_format": "%b %d %H:%M:%S",
			},
			want: &want{
				key:   "timestamp_regex",
				value: "(\\w{3} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})",
			},
		},
		"WithNonZeroPaddedMonthWord": {
			input: map[string]interface{}{
				"timestamp_format": "%b %-d %H:%M:%S",
			},
			want: &want{
				key:   "timestamp_regex",
				value: "(\\w{3} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})",
			},
		},
		"WithYearAsTwoDigits": {
			input: map[string]interface{}{
				"timestamp_format": "%b %-d %y %H:%M:%S",
			},
			want: &want{
				key:   "timestamp_regex",
				value: "(\\w{3} \\s{0,1}\\d{1,2} \\d{2} \\d{2}:\\d{2}:\\d{2})",
			},
		},
		"WithYearAsFourDigits": {
			input: map[string]interface{}{
				"timestamp_format": "%b %-d %Y %H:%M:%S",
			},
			want: &want{
				key:   "timestamp_regex",
				value: "(\\w{3} \\s{0,1}\\d{1,2} \\d{4} \\d{2}:\\d{2}:\\d{2})",
			},
		},
		"WithNoTimestampFormat": {
			input: map[string]interface{}{
				"timestamp": "foo",
			},
			want: &want{
				key:   "",
				value: "",
			},
		},
		"WithInvalidTimestampFormat": {
			input: map[string]interface{}{
				"timestamp_format": "foo",
			},
			want: &want{
				key:   "timestamp_regex",
				value: "(foo)",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			res, returnVal := regex.ApplyRule(testCase.input)
			require.NotNil(t, res)
			assert.Equal(t, res, testCase.want.key)
			assert.Equal(t, returnVal, testCase.want.value)
		})
	}
}

func TestTimestampLayoutxRule(t *testing.T) {
	layout := new(TimestampLayout)
	type want struct {
		key   string
		value interface{}
	}
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *want
		wantErr error
	}{
		"WithNonZeroPaddedOptions": {
			input: map[string]interface{}{
				"timestamp_format": "%-m %-d %H:%M:%S",
			},
			want: &want{
				key:   "timestamp_layout",
				value: []string{"1 _2 15:04:05", "01 _2 15:04:05"},
			},
		},
		"WithZeroPaddedOptions": {
			input: map[string]interface{}{
				"timestamp_format": "%m %d %H:%M:%S",
			},
			want: &want{
				key:   "timestamp_layout",
				value: []string{"01 _2 15:04:05", "1 _2 15:04:05"},
			},
		},
		"WithZeroPaddedMonthWord": {
			input: map[string]interface{}{
				"timestamp_format": "%b %d %H:%M:%S",
			},
			want: &want{
				key:   "timestamp_layout",
				value: []string{"Jan _2 15:04:05"},
			},
		},
		"WithNonZeroPaddedMonthWord": {
			input: map[string]interface{}{
				"timestamp_format": "%b %-d %H:%M:%S",
			},
			want: &want{
				key:   "timestamp_layout",
				value: []string{"Jan _2 15:04:05"},
			},
		},
		"WithYearAsTwoDigits": {
			input: map[string]interface{}{
				"timestamp_format": "%b %-d %y %H:%M:%S",
			},
			want: &want{
				key:   "timestamp_layout",
				value: []string{"Jan _2 06 15:04:05"},
			},
		},
		"WithYearAsFourDigits": {
			input: map[string]interface{}{
				"timestamp_format": "%b %-d %Y %H:%M:%S",
			},
			want: &want{
				key:   "timestamp_layout",
				value: []string{"Jan _2 2006 15:04:05"},
			},
		},
		"WithNoTimestampFormat": {
			input: map[string]interface{}{
				"timestamp": "foo",
			},
			want: &want{
				key:   "",
				value: "",
			},
		},
		"WithInvalidTimestampFormat": {
			input: map[string]interface{}{
				"timestamp_format": "foo",
			},
			want: &want{
				key:   "timestamp_layout",
				value: []string{"foo"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			res, returnVal := layout.ApplyRule(testCase.input)
			require.NotNil(t, res)
			assert.Equal(t, res, testCase.want.key)
			assert.Equal(t, returnVal, testCase.want.value)
		})
	}
}
