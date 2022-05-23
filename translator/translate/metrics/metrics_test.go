// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics

import (
	"encoding/json"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/stretchr/testify/assert"
)

func TestMetrics(t *testing.T) {
	m := new(Metrics)
	var input interface{}
	agent.Global_Config.Region = "auto"
	err := json.Unmarshal([]byte(`{"metrics":{}}`), &input)
	assert.NoError(t, err)
	_, actual := m.ApplyRule(input)
	expected := map[string]interface{}(
		map[string]interface{}{
			"outputs": map[string]interface{}{
				"cloudwatch": []interface{}{
					map[string]interface{}{
						"force_flush_interval": "60s",
						"namespace":            "CWAgent",
						"region":               "auto",
						"tagexclude":           []string{"metricPath"},
						"tagpass":              map[string][]string{"metricPath": []string{"metrics"}},
					},
				},
			},
		},
	)
	assert.Equal(t, expected, actual, "Expected to be equal")

}

func TestMetrics_Internal(t *testing.T) {
	m := new(Metrics)
	var input interface{}
	agent.Global_Config.Region = "auto"
	agent.Global_Config.Internal = true
	err := json.Unmarshal([]byte(`{"metrics":{}}`), &input)
	assert.NoError(t, err)
	_, actual := m.ApplyRule(input)
	expected := map[string]interface{}(
		map[string]interface{}{
			"outputs": map[string]interface{}{
				"cloudwatch": []interface{}{
					map[string]interface{}{
						"force_flush_interval": "60s",
						"namespace":            "CWAgent",
						"region":               "auto",
						"max_datums_per_call":  1000,
						"max_values_per_datum": 5000,
						"tagexclude":           []string{"metricPath"},
						"tagpass":              map[string][]string{"metricPath": []string{"metrics"}},
					},
				},
			},
		},
	)
	assert.Equal(t, expected, actual, "Expected to be equal")
	agent.Global_Config.Internal = false //reset
}

func TestMetrics_EndpointOverride(t *testing.T) {
	m := new(Metrics)
	var input interface{}
	agent.Global_Config.Region = "auto"
	err := json.Unmarshal([]byte(`{"metrics":{"endpoint_override":"https://monitoring-fips.us-east-1.amazonaws.com"}}`), &input)
	assert.NoError(t, err)
	_, actual := m.ApplyRule(input)
	expected := map[string]interface{}(
		map[string]interface{}{
			"outputs": map[string]interface{}{
				"cloudwatch": []interface{}{
					map[string]interface{}{
						"force_flush_interval": "60s",
						"namespace":            "CWAgent",
						"region":               "auto",
						"endpoint_override":    "https://monitoring-fips.us-east-1.amazonaws.com",
						"tagexclude":           []string{"metricPath"},
						"tagpass":              map[string][]string{"metricPath": []string{"metrics"}},
					},
				},
			},
		},
	)
	assert.Equal(t, expected, actual, "Expected to be equal")
}
