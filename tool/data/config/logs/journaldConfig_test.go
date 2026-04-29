// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestJournaldConfig_ToMap(t *testing.T) {
	conf := &JournaldConfig{
		LogGroup:        "system-logs",
		LogStream:       "{instance_id}",
		Units:           []string{"systemd", "kernel"},
		Filters: []*JournaldFilter{
			{
				Type:       "exclude",
				Expression: ".*debug.*",
			},
			{
				Type:       "include",
				Expression: ".*error.*",
			},
		},
		RetentionInDays: 7,
	}

	expectedVal := map[string]interface{}{
		"log_group_name":    "system-logs",
		"log_stream_name":   "{instance_id}",
		"units":             []string{"systemd", "kernel"},
		"filters": []map[string]interface{}{
			{
				"type":       "exclude",
				"expression": ".*debug.*",
			},
			{
				"type":       "include",
				"expression": ".*error.*",
			},
		},
		"retention_in_days": 7,
	}

	ctx := &runtime.Context{}
	actualKey, actualVal := conf.ToMap(ctx)

	assert.Equal(t, "", actualKey)
	assert.Equal(t, expectedVal, actualVal)
}

func TestJournaldConfig_ToMap_MinimalConfig(t *testing.T) {
	conf := &JournaldConfig{
		LogGroup:  "minimal-logs",
		LogStream: "{hostname}",
	}

	expectedVal := map[string]interface{}{
		"log_group_name":  "minimal-logs",
		"log_stream_name": "{hostname}",
	}

	ctx := &runtime.Context{}
	actualKey, actualVal := conf.ToMap(ctx)

	assert.Equal(t, "", actualKey)
	assert.Equal(t, expectedVal, actualVal)
}