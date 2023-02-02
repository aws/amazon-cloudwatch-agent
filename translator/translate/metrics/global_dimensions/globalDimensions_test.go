// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package global_dimensions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobalDimensions(t *testing.T) {
	e := new(globalDimensions)
	var input interface{}
	err := json.Unmarshal([]byte(`{
      "global_dimensions": {
				"Environment": "test",
				"Dimension": "value",
				"InvalidBecauseNoValue": "",
				"": "InvalidBecauseNoKey"
			}
    }`), &input)
	if err == nil {
		actualKey, actualValue := e.ApplyRule(input)
		expected := map[string]interface{}{
			"global_dimensions": map[string]interface{}{
				"Environment": "test",
				"Dimension":   "value",
			},
		}
		assert.Equal(t, expected, actualValue, "Expect values to be equal")
		assert.Equal(t, actualKey, "outputs", "Expect keys to be equal")
	} else {
		panic(err)
	}
}

func TestGlobalDimensionsNotProvided(t *testing.T) {
	e := new(globalDimensions)
	var input interface{}
	err := json.Unmarshal([]byte(`{
      "something_else": {
				"Environment": "test",
				"Dimension": "value"
			}
    }`), &input)

	assert.NoError(t, err)
	actualKey, actualValue := e.ApplyRule(input)
	assert.Equal(t, "", actualValue, "Expect value to be empty string")
	assert.Equal(t, "", actualKey, "Expect key to be empty string")
}

func TestGlobalDimensionsEmpty(t *testing.T) {
	e := new(globalDimensions)
	var input interface{}
	err := json.Unmarshal([]byte(`{
      "global_dimensions": {}
    }`), &input)

	assert.NoError(t, err)
	actualKey, actualValue := e.ApplyRule(input)
	assert.Equal(t, "", actualValue, "Expect value to be empty string")
	assert.Equal(t, "", actualKey, "Expect key to be empty string")
}

func TestGlobalDimensionsAllInvalid(t *testing.T) {
	e := new(globalDimensions)
	var input interface{}
	err := json.Unmarshal([]byte(`{
      "global_dimensions": {
				"InvalidBecauseNoValue": "",
				"": "InvalidBecauseNoKey"
			}
    }`), &input)

	assert.NoError(t, err)

	actualKey, actualValue := e.ApplyRule(input)
	assert.Equal(t, "", actualValue, "Expect value to be empty string")
	assert.Equal(t, "", actualKey, "Expect key to be empty string")
}
