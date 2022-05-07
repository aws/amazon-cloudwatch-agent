// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetWithSameKeyIfFound(t *testing.T) {
	var rawJsonString = `
{
  "itemA": "valueA",
  "itemB": "1",
  "itemC": "valueC",
  "tagC": {
    "key1": "value1",
    "key2": "value2"
  },
  "tagD": {
    "key3": "value3",
    "key4": "value4"
  }
}
`
	var input interface{}

	target_key_list := []string{"itemA", "itemB", "tagC"}
	var expected = map[string]interface{}{
		"itemA": "valueA",
		"itemB": "1",
		"tagC":  map[string]interface{}{"key1": "value1", "key2": "value2"},
	}
	var actual = map[string]interface{}{}
	err := json.Unmarshal([]byte(rawJsonString), &input)
	if err == nil {
		SetWithSameKeyIfFound(input, target_key_list, actual)
		assert.Equal(t, expected, actual)
	} else {
		panic(err)
	}
}

func TestSetWithCustomizedKeyIfFound(t *testing.T) {
	var rawJsonString = `
{
  "itemA": "valueA",
  "itemB": "1",
  "itemC": "valueC",
  "tagC": {
    "key1": "value1",
    "key2": "value2"
  },
  "tagD": {
    "key3": "value3",
    "key4": "value4"
  }
}
`
	var input interface{}

	targetKeyMap := map[string]string{"itemA": "itemAMapped", "itemB": "itemBMapped", "tagC": "tagCMapped"}
	var expected = map[string]interface{}{
		"itemAMapped": "valueA",
		"itemBMapped": "1",
		"tagCMapped":  map[string]interface{}{"key1": "value1", "key2": "value2"},
	}
	var actual = map[string]interface{}{}
	err := json.Unmarshal([]byte(rawJsonString), &input)
	if err == nil {
		SetWithCustomizedKeyIfFound(input, targetKeyMap, actual)
		assert.Equal(t, expected, actual)
	} else {
		panic(err)
	}
}
