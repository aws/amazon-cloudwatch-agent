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
	var rawJsonString = `
{
    "collect_list": [
      {
        "event_name": "System",
        "event_levels": [
          "INFORMATION",
          "CRITICAL"
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
			"log_group_class":   util.StandardLogGroupClass,
		},
		map[string]interface{}{
			"event_name":        "Application",
			"event_levels":      []interface{}{"4", "0", "5", "2"},
			"event_format":      "xml",
			"log_group_name":    "Application",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 1,
			"log_group_class":   "",
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

func TestDuplicateRetention(t *testing.T) {
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
			"log_group_name":    "System",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 3,
			"log_group_class":   util.InfrequentAccessLogGroupClass,
		},
		map[string]interface{}{
			"event_name":        "Application",
			"event_levels":      []interface{}{"4", "0", "5", "2"},
			"event_format":      "xml",
			"log_group_name":    "System",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 3,
			"log_group_class":   util.InfrequentAccessLogGroupClass,
		},
		map[string]interface{}{
			"event_name":        "Application",
			"event_levels":      []interface{}{"4", "0", "5", "2"},
			"event_format":      "xml",
			"log_group_name":    "System",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 3,
			"log_group_class":   util.InfrequentAccessLogGroupClass,
		},
	}

	var actual interface{}

	error := json.Unmarshal([]byte(rawJsonString), &input)
	if error == nil {
		_, actual = c.ApplyRule(input)
		assert.Equal(t, 0, len(translator.ErrorMessages))
		assert.Equal(t, expected, actual)
	} else {
		assert.Fail(t, error.Error())
	}
}

func TestConflictingRetention(t *testing.T) {
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
			"log_group_name":    "System",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 3,
			"log_group_class":   "",
		},
		map[string]interface{}{
			"event_name":        "Application",
			"event_levels":      []interface{}{"4", "0", "5", "2"},
			"event_format":      "xml",
			"log_group_name":    "System",
			"batch_read_size":   BatchReadSizeValue,
			"retention_in_days": 1,
			"log_group_class":   "",
		},
	}

	var actual interface{}

	error := json.Unmarshal([]byte(rawJsonString), &input)
	if error == nil {
		_, actual = c.ApplyRule(input)
		assert.GreaterOrEqual(t, 1, len(translator.ErrorMessages))
		assert.Equal(t, "Under path : /logs/logs_collected/windows_events/collect_list/ | Error : Different retention_in_days values can't be set for the same log group: system", translator.ErrorMessages[len(translator.ErrorMessages)-1])
		assert.Equal(t, expected, actual)
	} else {
		assert.Fail(t, error.Error())
	}
}
