// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEventConfig_ToMap(t *testing.T) {
	conf := &EventConfig{
		EventName:   "System",
		LogGroup:    "SystemGroup",
		LogStream:   "SystemStream",
		EventLevels: []string{"INFORMATION", "WARNING", "ERROR", "SUCCESS"},
	}
	ctx := &runtime.Context{}
	key, value := conf.ToMap(ctx)
	assert.Equal(t, "", key)
	assert.Equal(t, map[string]interface{}{
		"event_name":      "System",
		"event_levels":    []string{"INFORMATION", "WARNING", "ERROR", "SUCCESS"},
		"log_group_name":  "SystemGroup",
		"log_stream_name": "SystemStream"},
		value)
}
