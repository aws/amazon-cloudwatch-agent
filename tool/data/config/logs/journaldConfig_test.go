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
		LogGroup:  "JournaldGroup",
		LogStream: "JournaldStream",
		Filters: []*EventFilter{
			{Type: "exclude", Expression: "debug|trace"},
			{Type: "exclude", Expression: ".*verbose.*"},
		},
		Retention: 7,
	}
	ctx := &runtime.Context{}
	key, value := conf.ToMap(ctx)
	assert.Equal(t, "", key)
	assert.Equal(t, map[string]interface{}{
		"log_group_name":  "JournaldGroup",
		"log_stream_name": "JournaldStream",
		"filters": []map[string]interface{}{
			{"type": "exclude", "expression": "debug|trace"},
			{"type": "exclude", "expression": ".*verbose.*"},
		},
		"retention_in_days": 7},
		value)
}