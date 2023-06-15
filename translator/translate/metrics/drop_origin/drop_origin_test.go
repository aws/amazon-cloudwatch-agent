// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package drop_origin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDropOriginal(t *testing.T) {
	e := new(DropOrigin)
	//Check whether override default config
	var input interface{}
	err := json.Unmarshal([]byte(`{
	 			"metrics_collected": {
	 				"cpu": {
	 					"drop_original_metrics": ["cpu_usage_guest", "cpu_usage_idle"],
	 					"measurement": [
	 						"cpu_usage_guest",
							"cpu_usage_system",
							"cpu_usage_idle"
	 					]
	 				}
	 			}}`), &input)
	assert.NoError(t, err)
	actualKey, actualVal := e.ApplyRule(input)
	expectedKey := "drop_original_metrics"
	expectedVal := map[string][]string{
		"cpu": {"cpu_usage_guest", "cpu_usage_idle"},
	}

	assert.Equal(t, expectedKey, actualKey)
	assert.Equal(t, expectedVal, actualVal)

}

func TestDropMultipleOriginal(t *testing.T) {
	e := new(DropOrigin)
	//Check whether override default config
	var input interface{}
	err := json.Unmarshal([]byte(`{
	 			"metrics_collected": {
	 				"cpu": {
	 					"drop_original_metrics": ["cpu_usage_guest", "cpu_usage_idle"],
	 					"measurement": [
	 						"cpu_usage_guest",
							"cpu_usage_system",
							"cpu_usage_idle"
	 					]
	 				},
					"nvidia_gpu": {
	 					"drop_original_metrics": ["utilization_gpu", "temperature_gpu"],
						"measurement": [
							"utilization_gpu", 
							"temperature_gpu", 
							"power_draw", 
							"utilization_memory", 
							"fan_speed", 
							"memory_total", 
							"memory_used", 
							"memory_free", 
							"temperature_gpu"
						]
					}
	 			}}`), &input)
	assert.NoError(t, err)
	actualKey, actualVal := e.ApplyRule(input)
	expectedKey := "drop_original_metrics"
	expectedVal := map[string][]string{
		"cpu":        {"cpu_usage_guest", "cpu_usage_idle"},
		"nvidia_smi": {"utilization_gpu", "temperature_gpu"},
	}

	assert.Equal(t, expectedKey, actualKey)
	assert.Equal(t, expectedVal, actualVal)

}

func TestNotDropOriginal(t *testing.T) {
	e := new(DropOrigin)
	//Check whether override default config
	var input interface{}
	err := json.Unmarshal([]byte(`{
	 			"metrics_collected": {
	 				"cpu": {
	 					"drop_original_metrics": [],
	 					"measurement": [
	 						"cpu_usage_guest"
	 					]
	 				},
					"nvidia_gpu": {
						"measurement": [
							"memory_used"
						]
					}
	 			}}`), &input)
	assert.NoError(t, err)
	actualKey, actualVal := e.ApplyRule(input)

	assert.Equal(t, "", actualKey)
	assert.Equal(t, "", actualVal)
}

func TestDefaultDropOriginal(t *testing.T) {
	e := new(DropOrigin)
	//Check whether override default config
	var input interface{}
	err := json.Unmarshal([]byte(`{
	 			"metrics_collected": {
	 				"cpu": {
	 					"measurement": [
	 						"cpu_usage_guest"
	 					]
	 				}
	 			}}`), &input)
	assert.NoError(t, err)
	actualKey, actualVal := e.ApplyRule(input)

	assert.Equal(t, "", actualKey)
	assert.Equal(t, "", actualVal)
}
