// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statsd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatsD_HappyCase(t *testing.T) {
	obj := new(StatsD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"statsd": {
					"service_address": ":12345",
					"metrics_collection_interval": 5,
					"metrics_aggregation_interval": 30,
					"allowed_pending_messages": 10000,
					"templates": ["measurement.*"]
					}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"allowed_pending_messages": 10000,
			"service_address":          ":12345",
			"interval":                 "5s",
			"parse_data_dog_tags":      true,
			"tags":                     map[string]interface{}{"aws:AggregationInterval": "30s"},
			"templates":                []string{"measurement.*"},
		},
	}

	assert.Equal(t, expect, actual)
}

func TestStatsD_MinimumConfig(t *testing.T) {
	obj := new(StatsD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"statsd": {}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"service_address":     ":8125",
			"interval":            "10s",
			"parse_data_dog_tags": true,
			"tags":                map[string]interface{}{"aws:AggregationInterval": "60s"},
			"templates":           []string{},
		},
	}

	assert.Equal(t, expect, actual)
}

func TestStatsD_DisableAggregation(t *testing.T) {
	obj := new(StatsD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"statsd": {
					"metrics_aggregation_interval": 0
					}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"service_address":     ":8125",
			"interval":            "10s",
			"parse_data_dog_tags": true,
			"tags":                map[string]interface{}{"aws:StorageResolution": "true"},
			"templates":           []string{},
		},
	}

	assert.Equal(t, expect, actual)
}

func TestStatsD_MetricSeparator(t *testing.T) {
	obj := new(StatsD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"statsd": {
					"metric_separator": "."
					}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"service_address":     ":8125",
			"interval":            "10s",
			"parse_data_dog_tags": true,
			"tags":                map[string]interface{}{"aws:AggregationInterval": "60s"},
			"metric_separator":    ".",
			"templates":           []string{},
		},
	}

	assert.Equal(t, expect, actual)
}

func TestStatsD_Templates(t *testing.T) {
	obj := new(StatsD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"statsd": {
					"templates": ["hi"]
					}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"service_address":     ":8125",
			"interval":            "10s",
			"parse_data_dog_tags": true,
			"tags":                map[string]interface{}{"aws:AggregationInterval": "60s"},
			"templates":           []string{"hi"},
		},
	}

	assert.Equal(t, expect, actual)
}
