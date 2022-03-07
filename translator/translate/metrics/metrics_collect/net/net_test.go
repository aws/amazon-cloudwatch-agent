// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package net

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNet(t *testing.T) {
	n := new(Net)
	var input interface{}
	e := json.Unmarshal([]byte(`{"net":{"measurement": [
						"bytes_sent",
						"bytes_recv",
						"dummy_drop_in"]}}`), &input)
	if e == nil {
		_, actual := n.ApplyRule(input)
		expected := []interface{}{map[string]interface{}{
			"fieldpass": []string{"bytes_sent", "bytes_recv"},
			"tags":      map[string]interface{}{"report_deltas": "true"},
		}}
		assert.Equal(t, expected, actual, "Expected to be equal")
	} else {
		panic(e)
	}
}

func TestNetWithReportDeltaTrue(t *testing.T) {
	n := new(Net)
	var input interface{}
	e := json.Unmarshal([]byte(`{"net":{"measurement": [
						"bytes_sent",
						"bytes_recv",
						"dummy_drop_in"],"report_deltas":true}}`), &input)
	if e == nil {
		_, actual := n.ApplyRule(input)
		expected := []interface{}{map[string]interface{}{
			"fieldpass": []string{"bytes_sent", "bytes_recv"},
			"tags":      map[string]interface{}{"report_deltas": "true"},
		}}
		assert.Equal(t, expected, actual, "Expected to be equal")
	} else {
		panic(e)
	}
}

func TestNetWithReportDeltaFalse(t *testing.T) {
	n := new(Net)
	var input interface{}
	e := json.Unmarshal([]byte(`{"net":{"measurement": [
						"bytes_sent",
						"bytes_recv",
						"dummy_drop_in"],"report_deltas":false}}`), &input)
	if e == nil {
		_, actual := n.ApplyRule(input)
		expected := []interface{}{map[string]interface{}{
			"fieldpass": []string{"bytes_sent", "bytes_recv"},
		}}
		assert.Equal(t, expected, actual, "Expected to be equal")
	} else {
		panic(e)
	}
}
