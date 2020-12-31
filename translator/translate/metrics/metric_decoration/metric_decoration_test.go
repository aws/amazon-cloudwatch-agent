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
						{"name": "cpu_usage_idle", "rename": "CPU", "unit": "unit"},
						{"name": "cpu_usage_nice", "unit": "unit"},
						"cpu_usage_guest"
					]
				}
			}}`), &input)

	require.Nil(t, err)
	_, val := c.ApplyRule(input)
	expected := []interface{}{
		map[string]string{
			"rename":   "CPU",
			"unit":     "unit",
			"category": "cpu",
			"name":     "usage_idle",
		},
		map[string]string{
			"category": "cpu",
			"name":     "usage_nice",
			"unit":     "unit",
		},
	}
	assert.Equal(t, expected, val)
}
