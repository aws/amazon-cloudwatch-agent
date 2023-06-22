// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cpu

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Check the case when the input is in "cpu":{//specific configuration}
func TestCpuSpecificConfig(t *testing.T) {
	c := new(Cpu)
	//Check whether override default config
	var input interface{}
	err := json.Unmarshal([]byte(`{"cpu":{"metrics_collection_interval":"11s"}}`), &input)
	if err == nil {
		actualReturnKey, _ := c.ApplyRule(input)
		assert.Equal(t, "", actualReturnKey, "Expect to be equal")
	} else {
		panic(err)
	}
}

func TestNonCpuConfig(t *testing.T) {
	c := new(Cpu)
	var input interface{}
	err := json.Unmarshal([]byte(`{"NotCpu":{"foo":"bar"}}`), &input)

	if err == nil {
		actualKey, actualVal := c.ApplyRule(input)
		expectedKey := ""
		expectedVal := ""
		assert.Equal(t, expectedKey, actualKey, "ReturnKey should be empty")
		assert.Equal(t, expectedVal, actualVal, "ReturnVal should be empty")
	} else {
		panic(err)
	}
}

func TestInvalidMetrics(t *testing.T) {
	c := new(Cpu)
	var input interface{}
	err := json.Unmarshal([]byte(`{"cpu": {
					"resources": [
						"*"
					],
					"measurement": [
						"cpu_usage_idle_dummy",
						"cpu_usage_dummy_nice",
						"dummy_cpu_usage_guest"
					],
					"totalcpu": true,
					"metrics_collection_interval": "1s"
				}}`), &input)
	if err == nil {
		actualKey, _ := c.ApplyRule(input)
		assert.Equal(t, "", actualKey, "return key should be empty")
	} else {
		panic(err)
	}
}
