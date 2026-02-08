// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/util"
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

func TestApplyRule(t *testing.T) {
	c := new(CollectList)
	var rawJSONString = `
{
    "collect_list": [
      {
        "event_name": "System",
        "event_levels": [
          "INFORMATION",
          "CRITICAL"
        ],
        "event_ids": [
          100,
          120,
          300
        ],
        "log_group_name": "System",
        "log_group_class": "STANDARD"
      },
      {
        "event_name": "Application",
        "event_levels": [
          "INFORMATION",
          "VERBOSE",
          "ERROR"
        ],
        "event_ids": [
         4625,
         3568
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
			"event_ids":         []int{100, 120, 300},
			"log_group_name":    "System",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": -1,
			"log_group_class":   util.StandardLogGroupClass,
		},
		map[string]interface{}{
			"event_name":        "Application",
			"event_levels":      []interface{}{"4", "0", "5", "2"},
			"event_ids":         []int{4625, 3568},
			"event_format":      "xml",
			"log_group_name":    "Application",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 1,
			"log_group_class":   "",
		},
	}

	var actual interface{}

	err := json.Unmarshal([]byte(rawJSONString), &input)
	if err == nil {
		_, actual = c.ApplyRule(input)
		assert.Equal(t, expected, actual)
	} else {
		panic(err)
	}
}

func TestDuplicateRetention(t *testing.T) {
	c := new(CollectList)
	var rawJSONString = `
{
    "collect_list": [
      {
        "event_name": "System",
        "event_levels": [
          "INFORMATION",
          "CRITICAL"
        ],
        "event_ids": [
          100,
          120
        ],
        "log_group_name": "System",
		"retention_in_days": 3,
		"log_group_class": "INFREQUENT_ACCESS"
      },
      {
        "event_name": "Application",
        "event_levels": [
          "INFORMATION",
          "VERBOSE",
          "ERROR"
        ],
        "event_ids": [
          100,
          120
        ],
        "event_format": "xml",
        "log_group_name": "System",
		"retention_in_days": 3,
		"log_group_class": "INFREQUENT_ACCESS"
      },
      {
        "event_name": "Application",
        "event_levels": [
          "INFORMATION",
          "VERBOSE",
          "ERROR"
        ],
        "event_ids": [
          100,
          120
        ],
        "event_format": "xml",
        "log_group_name": "System",
		"retention_in_days": 3,
		"log_group_class": "INFREQUENT_ACCESS"
      }
    ]
}
`
	var input interface{}

	var expected = []interface{}{
		map[string]interface{}{
			"event_name":        "System",
			"event_levels":      []interface{}{"4", "0", "1"},
			"event_ids":         []int{100, 120},
			"log_group_name":    "System",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 3,
			"log_group_class":   util.InfrequentAccessLogGroupClass,
		},
		map[string]interface{}{
			"event_name":        "Application",
			"event_levels":      []interface{}{"4", "0", "5", "2"},
			"event_ids":         []int{100, 120},
			"event_format":      "xml",
			"log_group_name":    "System",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 3,
			"log_group_class":   util.InfrequentAccessLogGroupClass,
		},
		map[string]interface{}{
			"event_name":        "Application",
			"event_levels":      []interface{}{"4", "0", "5", "2"},
			"event_ids":         []int{100, 120},
			"event_format":      "xml",
			"log_group_name":    "System",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 3,
			"log_group_class":   util.InfrequentAccessLogGroupClass,
		},
	}

	var actual interface{}

	err := json.Unmarshal([]byte(rawJSONString), &input)
	if err == nil {
		_, actual = c.ApplyRule(input)
		assert.Equal(t, 0, len(translator.ErrorMessages))
		assert.Equal(t, expected, actual)
	} else {
		assert.Fail(t, err.Error())
	}
}

func TestConflictingRetention(t *testing.T) {
	c := new(CollectList)
	var rawJSONString = `
{
    "collect_list": [
      {
        "event_name": "System",
        "event_levels": [
          "INFORMATION",
          "CRITICAL"
        ],
        "event_ids": [
          100,
          120
        ],
        "log_group_name": "System",
        "retention_in_days": 3
      },
      {
        "event_name": "Application",
        "event_levels": [
          "INFORMATION",
          "VERBOSE",
          "ERROR"
        ],
        "event_ids": [
          100,
          120
        ],
        "event_format": "xml",
        "log_group_name": "System",
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
			"event_ids":         []int{100, 120},
			"log_group_name":    "System",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 3,
			"log_group_class":   "",
		},
		map[string]interface{}{
			"event_name":        "Application",
			"event_levels":      []interface{}{"4", "0", "5", "2"},
			"event_ids":         []int{100, 120},
			"event_format":      "xml",
			"log_group_name":    "System",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 1,
			"log_group_class":   "",
		},
	}

	var actual interface{}

	err := json.Unmarshal([]byte(rawJSONString), &input)
	if err == nil {
		_, actual = c.ApplyRule(input)
		assert.GreaterOrEqual(t, 1, len(translator.ErrorMessages))
		assert.Equal(t, "Under path : /logs/logs_collected/windows_events/collect_list/ | Error : Different retention_in_days values can't be set for the same log group: system", translator.ErrorMessages[len(translator.ErrorMessages)-1])
		assert.Equal(t, expected, actual)
	} else {
		assert.Fail(t, err.Error())
	}
}

func TestEventID(t *testing.T) {
	//Inputs
	rawJSONString := `{
		"collect_list": [{
			"event_name": "System",
			"event_ids": [100, 101, 102],
			"event_levels": ["ERROR", "CRITICAL"]
		}]
		}`

	var config interface{}
	err := json.Unmarshal([]byte(rawJSONString), &config)
	assert.NoError(t, err)

	//process new configutation
	c := new(CollectList)
	_, val := c.ApplyRule(config)

	// Verify event_ids in final configuration
	result := val.([]interface{})[0].(map[string]interface{})

	eventIDs, exists := result["event_ids"]
	assert.True(t, exists, "event_ids should exist in final configuration")
	assert.Equal(t, []int{100, 101, 102}, eventIDs)

}
