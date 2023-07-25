// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

func TestApplyLogFiltersRule(t *testing.T) {
	translator.ResetMessages()
	r := new(LogFilter)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"filters": [
			{"type": "include", "expression": "foo"},
			{"type": "exclude", "expression": "bar"}
		]
	}`), &input)
	assert.Nil(t, e)

	retKey, retVal := r.ApplyRule(input)
	assert.Equal(t, "filters", retKey)
	assert.NotNil(t, retVal)
	assert.Len(t, translator.ErrorMessages, 0)

	filters := retVal.([]interface{})
	assert.Len(t, filters, 2)
	filter1 := filters[0].(map[string]interface{})
	val, ok := filter1["type"]
	assert.True(t, ok)
	assert.Equal(t, "include", val)
	val, ok = filter1["expression"]
	assert.True(t, ok)
	assert.Equal(t, "foo", val)
	filter2 := filters[1].(map[string]interface{})
	val, ok = filter2["type"]
	assert.True(t, ok)
	assert.Equal(t, "exclude", val)
	val, ok = filter2["expression"]
	assert.True(t, ok)
	assert.Equal(t, "bar", val)
}

func TestApplyLogFiltersRuleMissingConfigInsideFilters(t *testing.T) {
	translator.ResetMessages()
	r := new(LogFilter)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"filters": [
			{"foo": "include", "bar": "foo"},
			{"type": "exclude"}
		]
	}`), &input)
	assert.Nil(t, e)
	retKey, retVal := r.ApplyRule(input)
	assert.Equal(t, "filters", retKey)
	assert.Nil(t, retVal)
	assert.Len(t, translator.ErrorMessages, 2)
}

func TestApplyLogFiltersRuleInvalidRegex(t *testing.T) {
	translator.ResetMessages()
	r := new(LogFilter)
	var input interface{}
	e := json.Unmarshal([]byte(`{
		"filters": [
			{"type": "exclude", "expression": "(?!re)"}
		]
	}`), &input)
	assert.Nil(t, e)
	_, retVal := r.ApplyRule(input)
	assert.Nil(t, retVal)
	assert.Len(t, translator.ErrorMessages, 1)
}
