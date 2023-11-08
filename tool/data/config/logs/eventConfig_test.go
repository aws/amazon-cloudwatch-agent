// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

func TestEventConfig_ToMap(t *testing.T) {
	conf := &EventConfig{
		EventName:     "System",
		LogGroup:      "SystemGroup",
		LogStream:     "SystemStream",
		LogGroupClass: util.InfrequentAccessLogGroupClass,
		EventLevels:   []string{"INFORMATION", "WARNING", "ERROR", "SUCCESS"},
		Retention:     1,
	}
	ctx := &runtime.Context{}
	key, value := conf.ToMap(ctx)
	assert.Equal(t, "", key)
	assert.Equal(t, map[string]interface{}{
		"event_name":        "System",
		"event_levels":      []string{"INFORMATION", "WARNING", "ERROR", "SUCCESS"},
		"log_group_name":    "SystemGroup",
		"log_stream_name":   "SystemStream",
		"log_group_class":   util.InfrequentAccessLogGroupClass,
		"retention_in_days": 1},
		value)
}
