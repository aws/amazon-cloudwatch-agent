// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows_events

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func TestApplyRule(t *testing.T) {
	w := new(WindowsEvent)
	var rawJsonString = `
{
	"windows_events": {
        "collect_list": [
          {
            "event_name": "System",
            "event_levels": [
              "INFORMATION",
              "SUCCESS"
            ],
            "log_group_name": "System",
            "log_stream_name": "System"
          }
        ]
      }
}
`
	var input interface{}

	var expected = map[string]interface{}{
		"windows_event_log": []interface{}{
			map[string]interface{}{
				"destination":       "cloudwatchlogs",
				"file_state_folder": "C:\\ProgramData\\Amazon\\AmazonCloudWatchAgent\\Logs\\state",
			},
		},
	}

	var actual interface{}

	err := json.Unmarshal([]byte(rawJsonString), &input)
	if err == nil {
		context.CurrentContext().SetOs(config.OS_TYPE_WINDOWS)
		_, actual = w.ApplyRule(input)
		assert.Equal(t, expected, actual)
	} else {
		panic(err)
	}
}
