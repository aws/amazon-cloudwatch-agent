// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
)

func TestJournald_ToMap(t *testing.T) {
	conf := new(Journald)
	
	// Add first journald config
	conf.AddJournald(
		[]string{"systemd", "kernel"}, 
		"system-logs", 
		"{instance_id}", 
		[]*JournaldFilter{
			{Type: "exclude", Expression: ".*debug.*"},
		}, 
		7,
	)
	
	// Add second journald config
	conf.AddJournald(
		[]string{"nginx", "apache2"}, 
		"application-logs", 
		"{instance_id}-apps", 
		[]*JournaldFilter{
			{Type: "include", Expression: ".*error.*"},
			{Type: "exclude", Expression: ".*trace.*"},
		}, 
		14,
	)

	expectedKey := "journald"
	expectedVal := map[string]interface{}{
		"collect_list": []map[string]interface{}{
			{
				"log_group_name":    "system-logs",
				"log_stream_name":   "{instance_id}",
				"units":             []string{"systemd", "kernel"},
				"filters": []map[string]interface{}{
					{
						"type":       "exclude",
						"expression": ".*debug.*",
					},
				},
				"retention_in_days": 7,
			},
			{
				"log_group_name":    "application-logs",
				"log_stream_name":   "{instance_id}-apps",
				"units":             []string{"nginx", "apache2"},
				"filters": []map[string]interface{}{
					{
						"type":       "include",
						"expression": ".*error.*",
					},
					{
						"type":       "exclude",
						"expression": ".*trace.*",
					},
				},
				"retention_in_days": 14,
			},
		},
	}

	ctx := &runtime.Context{}
	actualKey, actualVal := conf.ToMap(ctx)

	assert.Equal(t, expectedKey, actualKey)
	assert.Equal(t, expectedVal, actualVal)
}

func TestJournald_ToMap_SingleEntry(t *testing.T) {
	conf := new(Journald)
	
	conf.AddJournald(
		[]string{"ssh"}, 
		"security-logs", 
		"{hostname}-security", 
		nil, 
		30,
	)

	expectedKey := "journald"
	expectedVal := map[string]interface{}{
		"collect_list": []map[string]interface{}{
			{
				"log_group_name":    "security-logs",
				"log_stream_name":   "{hostname}-security",
				"units":             []string{"ssh"},
				"retention_in_days": 30,
			},
		},
	}

	ctx := &runtime.Context{}
	actualKey, actualVal := conf.ToMap(ctx)

	assert.Equal(t, expectedKey, actualKey)
	assert.Equal(t, expectedVal, actualVal)
}