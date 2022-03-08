// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package netstat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetStat(t *testing.T) {
	n := new(NetStat)
	var input interface{}
	e := json.Unmarshal([]byte(`{"netstat":{"measurement": [
						"tcp_established",
						"tcp_syn_sent",
						"tcp_close"]}}`), &input)
	if e == nil {
		_, actual := n.ApplyRule(input)
		expected := []interface{}{map[string]interface{}{
			"fieldpass": []string{"tcp_established", "tcp_syn_sent", "tcp_close"},
		}}
		assert.Equal(t, expected, actual, "Expected to be equal")
	}
}
