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
	err := json.Unmarshal([]byte(`{"net":{"measurement": [
						"bytes_sent",
						"bytes_recv",
						"dummy_drop_in"]}}`), &input)
	if err == nil {
		_, actual := n.ApplyRule(input)
		expected := []interface{}{map[string]interface{}{
			"fieldpass": []string{"bytes_sent", "bytes_recv"},
		}}
		assert.Equal(t, expected, actual, "Expected to be equal")
	} else {
		panic(err)
	}
}
