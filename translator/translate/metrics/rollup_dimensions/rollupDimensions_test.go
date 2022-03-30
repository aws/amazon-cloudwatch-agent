// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rollup_dimensions

import (
	"encoding/json"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/translator"

	"github.com/stretchr/testify/assert"
)

func TestRollupDimensions(t *testing.T) {
	r := new(rollupDimensions)
	var input interface{}
	e := json.Unmarshal([]byte(`{
      "aggregation_dimensions": [["ImageId"], ["InstanceId", "InstanceType"], ["d1"],[]]
    }`), &input)
	if e == nil {
		_, actual := r.ApplyRule(input)
		expected := map[string]interface{}{
			"rollup_dimensions": []interface{}{
				[]interface{}{"ImageId"},
				[]interface{}{"InstanceId", "InstanceType"},
				[]interface{}{"d1"},
				[]interface{}{}},
		}
		assert.Equal(t, expected, actual, "Expect to be equal")
	} else {
		panic(e)
	}
}

func TestInvalidRollupList(t *testing.T) {
	var tmp interface{}
	var actualVal interface{}
	testInputs := [][]byte{
		[]byte(`{
      "aggregation_dimensions":["ImageId", "InstanceId", "InstanceType"]
    }`),
		[]byte(`{
      "aggregation_dimensions":[1, 2, 3]
    }`),
		[]byte(`{
      "aggregation_dimensions":[[1, 2]]
    }`),
		[]byte(`{
      "aggregation_dimensions":[]
    }`),
		[]byte(`{
      "aggregation_dimensions":"rollup"
    }`),
		[]byte(`{
      "aggregation_dimensions":{"ImageId" : "1"}
    }`),
	}
	for _, testInput := range testInputs {
		e := json.Unmarshal(testInput, &tmp)
		if e != nil {
			panic(e)
		}
		if im, ok := tmp.(map[string]interface{}); ok {
			actualVal = im[SectionKey]
		} else {
			t.FailNow()
		}
		assert.Equal(t, false, isValidRollupList(actualVal), "Expect to be false")
	}
	assert.Equal(t, len(testInputs), len(translator.ErrorMessages), "Expect one Error message")
}

func TestValidRollupList(t *testing.T) {
	var input interface{}
	var actualVal interface{}

	e := json.Unmarshal([]byte(`{
      "aggregation_dimensions":[["ImageId"], ["InstanceId", "InstanceType"], ["d1"],[]]
    }`), &input)

	if im, ok := input.(map[string]interface{}); ok {
		actualVal = im[SectionKey]
	} else {
		t.FailNow()
	}

	if e == nil {
		assert.Equal(t, true, isValidRollupList(actualVal), "Expect to be true")
	} else {
		panic(e)
	}
}
