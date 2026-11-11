// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package dpdk

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/testutil"
)

func TestDefaultConfig(t *testing.T) {
	_, actual := testutil.UnmarshalAndApplyRule(t, `{"dpdk": {}}`, new(Dpdk))

	expected := []interface{}{map[string]interface{}{
		"socket_path":  "/var/run/dpdk/rte/dpdk_telemetry.v2",
		"device_types": []string{"ethdev"},
		"ethdev":       map[string]interface{}{"exclude_commands": []string{"/ethdev/link_status"}},
		"fieldpass":    []string{}},
	}
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestFullConfig(t *testing.T) {
	_, actual := testutil.UnmarshalAndApplyRule(t, `{"dpdk": {
					"socket_path": "/var/run/dpdk/rte/vpp_telemetry.v2",
					"device_types": ["ethdev", "rawdev"],
					"additional_commands": ["/l3fwd-power/stats"],
					"ethdev_exclude_commands": [],
					"metrics_include": [
						"pps_allowance_exceeded",
						"bw_in_allowance_exceeded",
						"bw_out_allowance_exceeded"
					],
					"append_dimensions":{
						"name":"sampleName"
					}
					}}`, new(Dpdk))

	expected := []interface{}{map[string]interface{}{
		"socket_path":         "/var/run/dpdk/rte/vpp_telemetry.v2",
		"device_types":        []string{"ethdev", "rawdev"},
		"additional_commands": []string{"/l3fwd-power/stats"},
		"ethdev":              map[string]interface{}{"exclude_commands": []string{}},
		"fieldpass":           []string{"pps_allowance_exceeded", "bw_in_allowance_exceeded", "bw_out_allowance_exceeded"},
		"tags":                map[string]interface{}{"name": "sampleName"},
	},
	}

	// compare marshaled values since unmarshalled values have type conflicts
	// the actual uses interface instead of expected string type
	// interface will be converted to string on marshall
	// this is going to be marshaled into toml not pogo
	marshalActual, err := json.Marshal(actual)
	assert.NoError(t, err)
	marshalExpected, err := json.Marshal(expected)
	assert.NoError(t, err)
	assert.Equal(t, string(marshalExpected), string(marshalActual), "Expected to be equal")
}
