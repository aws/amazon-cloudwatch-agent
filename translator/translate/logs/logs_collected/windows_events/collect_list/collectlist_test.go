// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyRule(t *testing.T) {
	c := new(CollectList)
	var rawJsonString = `
{
    "collect_list": [
      {
        "event_name": "System",
        "event_levels": [
          "INFORMATION",
          "CRITICAL"
        ],
        "log_group_name": "System"
      },
      {
        "event_name": "Application",
        "event_levels": [
          "INFORMATION",
          "VERBOSE",
          "ERROR"
        ],
        "event_format": "xml",
        "log_group_name": "Application",
		"retention_in_days": 1
      }
    ]
}
`
	var input interface{}

	var expected = []interface{}{
		map[string]interface{}{
			"event_name":        "System",
			"event_levels":      []interface{}{"4", "0", "1"},
			"log_group_name":    "System",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": -1,
		},
		map[string]interface{}{
			"event_name":        "Application",
			"event_levels":      []interface{}{"4", "0", "5", "2"},
			"event_format":      "xml",
			"log_group_name":    "Application",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 1,
		},
	}

	var actual interface{}

	err := json.Unmarshal([]byte(rawJsonString), &input)
	if err == nil {
		_, actual = c.ApplyRule(input)
		assert.Equal(t, expected, actual)
	} else {
		panic(err)
	}
}
