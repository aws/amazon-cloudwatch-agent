// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ethtool

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	d := new(Ethtool)
	var input interface{}
	err := json.Unmarshal([]byte(`{"ethtool": {
					}}`), &input)
	assert.NoError(t, err)
	_, actual := d.ApplyRule(input)

	expected := []interface{}{map[string]interface{}{
		"interface_include": []string{"*"},
		"fieldpass":         []string{}},
	}
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestFullConfig(t *testing.T) {
	d := new(Ethtool)
	var input interface{}
	err := json.Unmarshal([]byte(`{"ethtool": {
					"interface_include": [
						"eth0"
					],
					"interface_exclude": [
						"eth1"
					],
					"metrics_include": [
						"bw_in_allowance_exceeded"
					]
					}}`), &input)
	assert.NoError(t, err)
	_, actual := d.ApplyRule(input)

	expected := []interface{}{map[string]interface{}{
		"interface_include": []string{"eth0"},
		"interface_exclude": []string{"eth1"},
		"fieldpass":         []string{"bw_in_allowance_exceeded"},
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
