// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

func TestLogs_ToMap(t *testing.T) {
	expectedKey := "logs"
	expectedValue := map[string]interface{}{
		"logs_collected": map[string]interface{}{
			"files": map[string]interface{}{
				"collect_list": []map[string]interface{}{
					{
						"file_path":                "file1",
						"log_group_name":           "log_group_1",
						"timestamp_format":         "%H:%M:%S %y %b %d",
						"timezone":                 "UTC",
						"multi_line_start_pattern": "{timestamp_format}",
						"log_stream_name":          "{hostname}",
						"retention_in_days":        1,
						"log_group_class":          util.StandardLogGroupClass,
					},
					{
						"file_path":                "file2",
						"log_group_name":           "log_group_2",
						"timestamp_format":         "%H:%M:%S %y %b %d",
						"timezone":                 "UTC",
						"multi_line_start_pattern": "{timestamp_format}",
						"log_stream_name":          "{hostname}",
						"retention_in_days":        1,
						"log_group_class":          util.StandardLogGroupClass,
					},
				},
			},
		},
	}
	conf := new(Logs)
	conf.AddLogFile("file1", "log_group_1", "{hostname}", "%H:%M:%S %y %b %d", "UTC", "{timestamp_format}", "", 1, util.StandardLogGroupClass)
	conf.AddLogFile("file2", "log_group_2", "{hostname}", "%H:%M:%S %y %b %d", "UTC", "{timestamp_format}", "", 1, util.StandardLogGroupClass)
	ctx := &runtime.Context{}
	key, value := conf.ToMap(ctx)
	assert.Equal(t, expectedKey, key)
	assert.Equal(t, expectedValue, value)
}
