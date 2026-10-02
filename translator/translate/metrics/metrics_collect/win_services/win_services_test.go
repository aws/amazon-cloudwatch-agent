// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package win_services

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWinServices_MinimumConfig(t *testing.T) {
	obj := new(WinServices)
	var input interface{}
	err := json.Unmarshal([]byte(`{"win_services": {}}`), &input)
	assert.NoError(t, err)

	key, actual := obj.ApplyRule(input)
	assert.Equal(t, SectionKey, key)

	expect := []interface{}{
		map[string]interface{}{},
	}
	assert.Equal(t, expect, actual)
}

func TestWinServices_FullConfig(t *testing.T) {
	obj := new(WinServices)
	var input interface{}
	err := json.Unmarshal([]byte(`{"win_services": {
		"service_names": ["AmazonSSMAgent", "Spooler", "Win*"],
		"excluded_service_names": ["WinRM"],
		"measurement": ["state", "startup_mode"],
		"metrics_collection_interval": 60,
		"append_dimensions": {"Environment": "prod"}
	}}`), &input)
	assert.NoError(t, err)

	key, actual := obj.ApplyRule(input)
	assert.Equal(t, SectionKey, key)

	expect := []interface{}{
		map[string]interface{}{
			"service_names":          []string{"AmazonSSMAgent", "Spooler", "Win*"},
			"excluded_service_names": []string{"WinRM"},
			"fieldpass":              []string{"state", "startup_mode"},
			"interval":               "60s",
			"tags":                   map[string]interface{}{"Environment": "prod"},
		},
	}

	marshalActual, err := json.Marshal(actual)
	assert.NoError(t, err)
	marshalExpected, err := json.Marshal(expect)
	assert.NoError(t, err)
	assert.Equal(t, string(marshalExpected), string(marshalActual))
}

func TestWinServices_MissingSection(t *testing.T) {
	obj := new(WinServices)
	var input interface{}
	err := json.Unmarshal([]byte(`{"cpu": {}}`), &input)
	assert.NoError(t, err)

	key, actual := obj.ApplyRule(input)
	assert.Equal(t, "", key)
	assert.Equal(t, "", actual)
}
