// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

func TestConfig_ToMap(t *testing.T) {
	conf := &Config{
		FilePath:              "/var/log/messages",
		LogGroup:              "/var/log/messages",
		LogGroupClass:         util.StandardLogGroupClass,
		TimestampFormat:       "%H:%M:%S %y %b %d",
		Timezone:              "UTC",
		MultiLineStartPattern: "{timestamp_format}",
	}
	ctx := &runtime.Context{}
	key, value := conf.ToMap(ctx)
	assert.Equal(t, "", key)
	assert.Equal(t, map[string]interface{}{
		"file_path":                "/var/log/messages",
		"log_group_name":           "/var/log/messages",
		"log_group_class":          util.StandardLogGroupClass,
		"timestamp_format":         "%H:%M:%S %y %b %d",
		"timezone":                 "UTC",
		"multi_line_start_pattern": "{timestamp_format}",
	},
		value)
}
