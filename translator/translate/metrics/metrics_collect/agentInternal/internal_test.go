// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agentInternal

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInternal(t *testing.T) {
	i := new(Internal)
	var input interface{}
	e := json.Unmarshal([]byte(`{"internal": {"measurement": [
						"memstats_alloc_bytes",
						"internal_mem_dummy"]}}`), &input)
	if e == nil {
		_, actual := i.ApplyRule(input)
		expected := []interface{}{map[string]interface{}{
			"fieldpass": []string{"memstats_alloc_bytes"},
		},
		}
		assert.Equal(t, expected, actual, "Expected to be equal")
	} else {
		panic(e)
	}
}
