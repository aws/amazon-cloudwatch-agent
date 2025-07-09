//Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

func TestApplyEventLogFiltersRule(t *testing.T) {
	translator.ResetMessages()
	r := new(WindowsEventFilter)
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

func TestApplyEventLogFiltersRuleMissingConfigInsideFilters(t *testing.T) {
	translator.ResetMessages()
	r := new(WindowsEventFilter)
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

func TestApplyEventLogFiltersRuleInvalidRegex(t *testing.T) {
	translator.ResetMessages()
	r := new(WindowsEventFilter)
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

func TestApplyEventLogFiltersRuleWithComplexRegex(t *testing.T) {
	translator.ResetMessages()
	r := new(WindowsEventFilter)
	var input interface{}
	e := json.Unmarshal([]byte(`{
        "filters": [
            {"type": "include", "expression": "EventID:(1001|1002|1003)"},
            {"type": "exclude", "expression": "Source:.*Test.*"},
            {"type": "include", "expression": "Level:[1-3]"}
        ]
    }`), &input)
	assert.Nil(t, e)

	retKey, retVal := r.ApplyRule(input)
	assert.Equal(t, "filters", retKey)
	assert.NotNil(t, retVal)
	assert.Len(t, translator.ErrorMessages, 0)

	filters := retVal.([]interface{})
	assert.Len(t, filters, 3)

	filter1 := filters[0].(map[string]interface{})
	assert.Equal(t, "include", filter1["type"])
	assert.Equal(t, "EventID:(1001|1002|1003)", filter1["expression"])
}
func TestApplyEventLogFiltersRuleEdgeCaseRegex(t *testing.T) {
	translator.ResetMessages()
	r := new(WindowsEventFilter)
	var input interface{}
	e := json.Unmarshal([]byte(`{
        "filters": [
            {"type": "include", "expression": "^EventID:1001$"},
            {"type": "exclude", "expression": "\\\\Server\\\\Path"},
            {"type": "include", "expression": ".*"},
            {"type": "exclude", "expression": ""}
        ]
    }`), &input)
	assert.Nil(t, e)

	retKey, retVal := r.ApplyRule(input)
	assert.Equal(t, "filters", retKey)
	require.NotNil(t, retVal)

	filters := retVal.([]interface{})
	assert.Len(t, filters, 3)
	
	// Verify that empty expression is filtered out
	expectedExpressions := []string{"^EventID:1001$", "\\\\Server\\\\Path", ".*"}
	for i, filter := range filters {
		filterMap := filter.(map[string]interface{})
		assert.Equal(t, expectedExpressions[i], filterMap["expression"])
	}
}

func TestApplyEventLogFiltersRuleInvalidRegexPatterns(t *testing.T) {
	translator.ResetMessages()
	r := new(WindowsEventFilter)

	testCases := []string{
		`{"filters": [{"type": "include", "expression": "["}]}`,          // Unclosed bracket
		`{"filters": [{"type": "include", "expression": "*"}]}`,          // Invalid quantifier
		`{"filters": [{"type": "include", "expression": "(?P<>test)"}]}`, // Invalid named group
	}

	for _, testCase := range testCases {
		translator.ResetMessages()
		var input interface{}
		e := json.Unmarshal([]byte(testCase), &input)
		assert.Nil(t, e)

		_, retVal := r.ApplyRule(input)
		assert.Nil(t, retVal, "Expected nil for invalid regex")
		assert.Len(t, translator.ErrorMessages, 1)
	}
}
