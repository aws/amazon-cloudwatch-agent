// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collected

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectD_HappyCase(t *testing.T) {
	obj := new(CollectD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"collectd": {
		"service_address": "udp://127.0.0.1:123",
		"name_prefix": "collectd_prefix_",
		"collectd_auth_file": "/etc/collectd/_auth_file",
		"collectd_security_level": "none",
		"collectd_typesdb": ["/usr/share/collectd/types.db", "/custom_location/types.db"],
		"metrics_aggregation_interval": 30
	}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"data_format":             "collectd",
			"service_address":         "udp://127.0.0.1:123",
			"name_prefix":             "collectd_prefix_",
			"collectd_auth_file":      "/etc/collectd/_auth_file",
			"collectd_security_level": "none",
			"collectd_typesdb":        []interface{}{"/usr/share/collectd/types.db", "/custom_location/types.db"},
			"tags":                    map[string]interface{}{"aws:AggregationInterval": "30s"},
		},
	}

	assert.Equal(t, expect, actual)
}

func TestCollectD_MinimumConfig(t *testing.T) {
	obj := new(CollectD)
	var input interface{}
	err := json.Unmarshal([]byte(`{"collectd": {}}`), &input)
	assert.NoError(t, err)

	_, actual := obj.ApplyRule(input)

	expect := []interface{}{
		map[string]interface{}{
			"data_format":             "collectd",
			"service_address":         "udp://127.0.0.1:25826",
			"name_prefix":             "collectd_",
			"collectd_auth_file":      "/etc/collectd/auth_file",
			"collectd_security_level": "encrypt",
			"collectd_typesdb":        []interface{}{"/usr/share/collectd/types.db"},
			"tags":                    map[string]interface{}{"aws:AggregationInterval": "60s"},
		},
	}

	assert.Equal(t, expect, actual)
}
