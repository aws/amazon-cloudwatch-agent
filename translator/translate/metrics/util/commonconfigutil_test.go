// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessLinuxCommonConfigNoValidMetrics(t *testing.T) {
	var input interface{}
	result := map[string]interface{}{}
	err := json.Unmarshal([]byte(`{
					"resources": [
						"*"
					],
					"measurement": [
						"cpu_usage_idle_dummy",
						"cpu_usage_dummy_nice",
						"dummy_cpu_usage_guest"
					],
					"totalcpu": true,
					"metrics_collection_interval": 1
				}`), &input)
	if err == nil {
		hasValidMetrics := ProcessLinuxCommonConfig(input, "cpu", "", result)
		assert.False(t, hasValidMetrics, "Shouldn't return any valid metrics")
	} else {
		panic(err)
	}
}

func TestProcessLinuxCommonConfigHappy(t *testing.T) {
	var input interface{}
	actualResult := map[string]interface{}{}
	err := json.Unmarshal([]byte(`{
					"resources": [
						"*"
					],
					"measurement": [
						"usage_idle",
						"usage_nice",
						"dummy_cpu_usage_guest"
					],
					"totalcpu": true,
					"metrics_collection_interval": 1
				}`), &input)
	if err == nil {
		hasValidMetrics := ProcessLinuxCommonConfig(input, "cpu", "", actualResult)
		expectedResult := map[string]interface{}{
			"fieldpass": []string{"usage_idle", "usage_nice"},
			"interval":  "1s",
			"tags":      map[string]interface{}{"aws:StorageResolution": "true"},
		}
		assert.True(t, hasValidMetrics, "Should return valid metrics")
		assert.Equal(t, expectedResult, actualResult, "should be equal")
	} else {
		panic(err)
	}
}
