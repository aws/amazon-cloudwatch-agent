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
					"metrics_aggregation_interval": 30
					}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"service_address":     ":12345",
			"interval":            "5s",
			"parse_data_dog_tags": true,
			"tags":                map[string]interface{}{"aws:AggregationInterval": "30s"},
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
		},
	}

	assert.Equal(t, expect, actual)
}
