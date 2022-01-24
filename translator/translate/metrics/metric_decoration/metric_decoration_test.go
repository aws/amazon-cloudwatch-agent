// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metric_decoration

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/stretchr/testify/assert"
)
//Check the case when the input is in "cpu":{//specific configuration}
func TestMetricDecoration_ApplyRule(t *testing.T) {
	c := new(MetricDecoration)
	//Check whether override default config
	var input interface{}
	err := json.Unmarshal([]byte(`{
			"metrics_collected": {
				"cpu": {
					"measurement": [
						{"name": "cpu_usage_idle", "rename": "CPU", "unit": "Percent"},
						{"name": "cpu_usage_nice", "unit": "Percent"},
						"cpu_usage_guest"
					]
				}
			}}`), &input)

	require.Nil(t, err)
	key, val := c.ApplyRule(input)
	expected := []interface{}{
		map[string]string{
			"rename":   "CPU",
			"unit":     "Percent",
			"category": "cpu",
			"name":     "usage_idle",
		},
		map[string]string{
			"category": "cpu",
			"name":     "usage_nice",
			"unit":     "Percent",
		},
		// cpu_usage_guest will not translated into anything since there is no metric decoration configured for it.
	}
	assert.Equal(t, "metric_decoration", key)
	assert.Equal(t, expected, val)
}

//Check the case when the input is in "nvidia_gpu":{//specific configuration}
func TestMetricDecoration_plugin_with_alias_ApplyRule(t *testing.T) {
	c := new(MetricDecoration)
	//Check whether override default config
	var input interface{}
	err := json.Unmarshal([]byte(`{
			"metrics_collected": {
				"nvidia_gpu": {
					"measurement": [
				  		{"name": "utilization_gpu", "rename": "gpu_usage", "unit": "Percent"},
						"temperature_gpu",
						{"name":"memory_total", "unit": "Bytes"}
					]
				}
			}}`), &input)

	require.Nil(t, err)
	_, val := c.ApplyRule(input)
	expected := []interface{}{
		map[string]string{
			"rename":   "gpu_usage",
			"unit":     "Percent",
			"category": "nvidia_smi",
			"name":     "utilization_gpu",
		},
		map[string]string{
			"category": "nvidia_smi",
			"name":     "memory_total",
			"unit":     "Bytes",
		},
		// temperature_gpu will not translated into anything since there is no metric decoration configured for it.
	}
	assert.Equal(t, expected, val)
}
