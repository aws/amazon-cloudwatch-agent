// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package swap

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Check the case when the input is in "swap":{//specific configuration}
func TestSwapSpecificConfig(t *testing.T) {
	s := new(Swap)
	var input interface{}
	e := json.Unmarshal([]byte(`{"swap":{"metrics_collection_interval":"10s"}}`), &input)
	if e == nil {
		actualReturnKey, _ := s.ApplyRule(input)
		assert.Equal(t, "", actualReturnKey, "return key should be empty")
	}

	var input1 interface{}
	e = json.Unmarshal([]byte(`{"swap":{"measurement": ["used","free"]}}`), &input1)
	if e == nil {
		_, actualVal := s.ApplyRule(input1)
		expectedVal := []interface{}{map[string]interface{}{
			"fieldpass": []string{"used", "free"},
		},
		}
		assert.Equal(t, expectedVal, actualVal, "Expect to be equal")
	} else {
		panic(e)
	}
}
