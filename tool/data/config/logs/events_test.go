// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

func TestEvents_ToMap(t *testing.T) {
	conf := new(Events)
	conf.AddWindowsEvent("EN1", "LG1", "LS1", "", []string{"ERROR", "SUCCESS"}, 1, util.InfrequentAccessLogGroupClass)
	conf.AddWindowsEvent("EN2", "LG2", "LS2", "xml", []string{"ERROR"}, 1, util.InfrequentAccessLogGroupClass)

	ctx := &runtime.Context{}
	actualkey, actualValue := conf.ToMap(ctx)

	expectedKey := "windows_events"
	expectedVal := map[string]interface{}{
		"collect_list": []map[string]interface{}{
			{"event_name": "EN1", "event_levels": []string{"ERROR", "SUCCESS"}, "log_group_name": "LG1", "log_stream_name": "LS1", "retention_in_days": 1, "log_group_class": util.InfrequentAccessLogGroupClass},
			{"event_name": "EN2", "event_levels": []string{"ERROR"}, "log_group_name": "LG2", "log_stream_name": "LS2", "event_format": "xml", "retention_in_days": 1, "log_group_class": util.InfrequentAccessLogGroupClass},
		},
	}
	assert.Equal(t, expectedKey, actualkey)
	assert.Equal(t, expectedVal, actualValue)
}
