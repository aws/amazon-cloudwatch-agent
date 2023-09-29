// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyTimestampFormatRule(t *testing.T) {
	r := new(TimestampRegax)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"timestamp_format": "%b %d %H:%M:%S"
	}`), &input)
	if e == nil {
		actualReturnKey, retVal := r.ApplyRule(input)
		assert.Equal(t, "timestamp_regex", actualReturnKey)
		assert.NotNil(t, retVal)
	} else {
		panic(e)
	}
}

func TestApplyInvalidTimestampFormatRule(t *testing.T) {
	regex := new(TimestampRegax)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"timestamp_format": "foo"
	}`), &input)
	if e == nil {
		actualReturnKey, retVal := regex.ApplyRule(input)
		assert.Equal(t, "timestamp_regex", actualReturnKey)
		assert.Equal(t, "(foo)", retVal)
	} else {
		panic(e)
	}
}

func TestApplyTimestampFormatLayoutRule(t *testing.T) {
	layout := new(TimestampLayout)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"timestamp_format": "%b %d %H:%M:%S"
	}`), &input)
	if e == nil {
		actualReturnKey, retVal := layout.ApplyRule(input)
		assert.Equal(t, "timestamp_layout", actualReturnKey)
		assert.NotNil(t, retVal)
		assert.Equal(t, "Jan 02 15:04:05", retVal)

	} else {
		panic(e)
	}
}

func TestApplyInvalidTimestampFormatLayoutRule(t *testing.T) {
	layout := new(TimestampLayout)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"timezone": "UTC"
	}`), &input)
	if e == nil {
		actualReturnKey, retVal := layout.ApplyRule(input)
		assert.Equal(t, "", actualReturnKey)
		assert.Equal(t, "", retVal)
	} else {
		panic(e)
	}
}

func TestApplyTimestampFormatZeroPaddingRule(t *testing.T) {
	// compare zero padding and not zero padding options are
	// translating as the same regex for %d/%-d and %m/%-m
	nonZeroRegax := new(TimestampRegax)
	zeroRegax := new(TimestampRegax)

	var non_zero interface{}
	var zero interface{}

	e := json.Unmarshal([]byte(`{
			"timestamp": "%-m %-d %H:%M:%S"
	}`), &non_zero)

	f := json.Unmarshal([]byte(`{
			"timestamp": "%m %d %H:%M:%S"
	}`), &zero)

	if (e == nil) || (f == nil) {
		zeroActualReturnKey, zeroRetVal := zeroRegax.ApplyRule(zero)
		nonZeroActualReturnKey, nonZeroRetVal := nonZeroRegax.ApplyRule(non_zero)
		assert.Equal(t, "timestamp_regex", zeroActualReturnKey)
		assert.Equal(t, zeroActualReturnKey, nonZeroActualReturnKey)
		assert.NotNil(t, zeroRetVal)
		assert.NotNil(t, nonZeroRetVal)
		assert.Equal(t, "(\\s{0,1}\\d{1,2} \\s{0,1}\\d{1,2} \\d{2}:\\d{2}:\\d{2})", zeroRetVal)
		assert.Equal(t, zeroRetVal, nonZeroRetVal)

	} else {
		panic(e)
	}
}
