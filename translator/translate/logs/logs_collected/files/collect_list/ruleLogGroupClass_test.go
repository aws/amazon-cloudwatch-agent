// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package collect_list

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

func TestApplyLogGroupClassRule(t *testing.T) {
	r := new(LogGroupClass)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"log_group_class": "Infrequent_access"
	}`), &input)
	if e == nil {
		actualReturnKey, actualReturnVal := r.ApplyRule(input)
		assert.Equal(t, "log_group_class", actualReturnKey)
		assert.Equal(t, util.InfrequentAccessLogGroupClass, actualReturnVal)
	} else {
		panic(e)
	}
}

// Since retention can only be set to specific numbers (1,3,5,7...),
// test to make sure other numbers are invalid (and set to -1)
func TestInvalidLogGroupClass(t *testing.T) {
	r := new(LogGroupClass)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"log_group_class": "invalidValue"
	}`), &input)
	if e == nil {
		actualReturnKey, actualReturnValue := r.ApplyRule(input)
		assert.Equal(t, "log_group_class", actualReturnKey)
		assert.Equal(t, "", actualReturnValue)
	} else {
		panic(e)
	}
}

func TestInvalidTypeLogGroupClass(t *testing.T) {
	r := new(LogGroupClass)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"log_group_class": 5
	}`), &input)
	if e == nil {
		actualReturnKey, actualReturnValue := r.ApplyRule(input)
		assert.Equal(t, "log_group_class", actualReturnKey)
		assert.Equal(t, "", actualReturnValue)
	} else {
		panic(e)
	}
}

func TestEmptyLogGroupClass(t *testing.T) {
	r := new(LogGroupClass)
	var input interface{}
	e := json.Unmarshal([]byte(`{
			"log_group_class": ""
	}`), &input)
	if e == nil {
		actualReturnKey, actualReturnValue := r.ApplyRule(input)
		assert.Equal(t, "log_group_class", actualReturnKey)
		assert.Equal(t, "", actualReturnValue)
	} else {
		panic(e)
	}
}
