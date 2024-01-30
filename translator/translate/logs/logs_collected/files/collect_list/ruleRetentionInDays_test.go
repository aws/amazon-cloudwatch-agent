// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyRetentionInDaysRule(t *testing.T) {
	r := new(RetentionInDays)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"retention_in_days": 5
	}`), &input)
	if e == nil {
		actualReturnKey, actualReturnValue := r.ApplyRule(input)
		assert.Equal(t, "retention_in_days", actualReturnKey)
		assert.Equal(t, 5, actualReturnValue)
	} else {
		panic(e)
	}
}

// Since retention can only be set to specific numbers (1,3,5,7...),
// test to make sure other numbers are invalid (and set to -1)
func TestRetentionInvalidNumberOfDays(t *testing.T) {
	r := new(RetentionInDays)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"retention_in_days": 2
	}`), &input)
	if e == nil {
		actualReturnKey, actualReturnValue := r.ApplyRule(input)
		assert.Equal(t, "retention_in_days", actualReturnKey)
		assert.Equal(t, -1, actualReturnValue)
	} else {
		panic(e)
	}
}

func TestRetentionInvalidInputType(t *testing.T) {
	r := new(RetentionInDays)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"retention_in_days": "invalid string input"
	}`), &input)
	if e == nil {
		actualReturnKey, actualReturnValue := r.ApplyRule(input)
		assert.Equal(t, "retention_in_days", actualReturnKey)
		assert.Equal(t, -1, actualReturnValue)
	} else {
		panic(e)
	}
}
