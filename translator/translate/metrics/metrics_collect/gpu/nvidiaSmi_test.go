// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gpu

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Check the case when the input is in "nvidia_gpu":{//specific configuration}
func TestSpecificConfig(t *testing.T) {
	n := new(NvidiaSmi)
	var input interface{}
	err := json.Unmarshal([]byte(`{"nvidia_gpu":{"measurement": [
						"utilization_gpu",
						"temperature_gpu"
					]}}`), &input)
	if err == nil {
		_, actualVal := n.ApplyRule(input)
		expectedVal := []interface{}{map[string]interface{}{
			"fieldpass":  []string{"utilization_gpu", "temperature_gpu"},
			"tagexclude": []string{"compute_mode", "pstate", "uuid"},
		},
		}
		assert.Equal(t, expectedVal, actualVal, "Expect to be equal")
	} else {
		panic(err)
	}
}

func TestNoFieldConfig(t *testing.T) {
	n := new(NvidiaSmi)
	var input interface{}
	var e = json.Unmarshal([]byte(`{"nvidia_gpu":{"metrics_collection_interval":"60s"}}`), &input)
	if e == nil {
		actualReturnKey, _ := n.ApplyRule(input)
		assert.Equal(t, "", actualReturnKey, "return key should be empty")
	} else {
		panic(e)
	}
}

func TestFullConfig(t *testing.T) {
	n := new(NvidiaSmi)
	var input interface{}
	err := json.Unmarshal([]byte(`{"nvidia_gpu":{"measurement": [
						"utilization_gpu", 
						"temperature_gpu", 
						"power_draw", 
						"utilization_memory", 
						"fan_speed", 
						"memory_total", 
						"memory_used", 
						"memory_free", 
						"temperature_gpu", 
						"pcie_link_gen_current", 
						"pcie_link_width_current",
						"encoder_stats_session_count", 
						"encoder_stats_average_fps", 
						"encoder_stats_average_latency",
						"clocks_current_graphics", 
						"clocks_current_sm", 
						"clocks_current_memory", 
						"clocks_current_video"
					]}}`), &input)
	if err == nil {
		_, actualVal := n.ApplyRule(input)
		expectedVal := []interface{}{map[string]interface{}{
			"fieldpass": []string{"utilization_gpu", "temperature_gpu", "power_draw", "utilization_memory",
				"fan_speed", "memory_total", "memory_used", "memory_free", "temperature_gpu", "pcie_link_gen_current",
				"pcie_link_width_current", "encoder_stats_session_count", "encoder_stats_average_fps",
				"encoder_stats_average_latency", "clocks_current_graphics", "clocks_current_sm",
				"clocks_current_memory", "clocks_current_video"},
			"tagexclude": []string{"compute_mode", "pstate", "uuid"},
		},
		}
		assert.Equal(t, expectedVal, actualVal, "Expect to be equal")
	} else {
		panic(err)
	}
}

func TestInvalidMetrics(t *testing.T) {
	c := new(NvidiaSmi)
	var input interface{}
	e := json.Unmarshal([]byte(`{"nvidia_gpu": {
					"measurement": [
						"invalid_utilization_gpu",
						"invalid_temperature_gpu",
						"dummy_invalid_field_name"
					],
					"metrics_collection_interval": "1s"
				}}`), &input)
	if e == nil {
		actualKey, _ := c.ApplyRule(input)
		assert.Equal(t, "", actualKey, "return key should be empty")
	} else {
		panic(e)
	}
}

func TestNonGpuConfig(t *testing.T) {
	c := new(NvidiaSmi)
	var input interface{}
	err := json.Unmarshal([]byte(`{"nvidia_smi":{"foo":"bar"}}`), &input)

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
