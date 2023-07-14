// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mem

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Check the case when the input is in "mem":{//specific configuration}
func TestMemSpecificConfig(t *testing.T) {
	m := new(Mem)
	//Check whether provide specific config
	var input interface{}
	err := json.Unmarshal([]byte(`{"mem":{"metrics_collection_interval":"60s"}}`), &input)
	if err == nil {
		actualReturnKey, _ := m.ApplyRule(input)
		assert.Equal(t, "", actualReturnKey, "return key should be empty")
	} else {
		panic(err)
	}

	var input1 interface{}
	err = json.Unmarshal([]byte(`{"mem":{"measurement": [
						"free",
						"total"
					]}}`), &input1)
	if err == nil {
		_, actualVal := m.ApplyRule(input1)
		expectedVal := []interface{}{map[string]interface{}{
			"fieldpass": []string{"free", "total"},
		},
		}
		assert.Equal(t, expectedVal, actualVal, "Expect to be equal")
	} else {
		panic(err)
	}
}
