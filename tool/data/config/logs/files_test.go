// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/stretchr/testify/assert"
)

func TestFiles_ToMap(t *testing.T) {
	conf := new(Files)

	conf.AddLogFile("/var/log", "lg1", "ls1", "timeStamp1", "utc", "p1", "utf-8", 1)
	conf.AddLogFile("/var/message", "lg2", "ls2", "timeStamp2", "pst", "p2", "", 1)

	expectedKey := "files"
	expectedVal := map[string]interface{}{
		"collect_list": []map[string]interface{}{
			{
				"multi_line_start_pattern": "p1",
				"log_stream_name":          "ls1",
				"file_path":                "/var/log",
				"log_group_name":           "lg1",
				"timestamp_format":         "timeStamp1",
				"timezone":                 "utc",
				"encoding":                 "utf-8",
				"retention_in_days":        1,
			},
			{
				"multi_line_start_pattern": "p2",
				"log_stream_name":          "ls2",
				"file_path":                "/var/message",
				"log_group_name":           "lg2",
				"timestamp_format":         "timeStamp2",
				"timezone":                 "pst",
				"retention_in_days":        1,
			},
		},
	}

	ctx := &runtime.Context{}
	actualKey, actualVal := conf.ToMap(ctx)

	assert.Equal(t, expectedKey, actualKey)
	assert.Equal(t, expectedVal, actualVal)

}
