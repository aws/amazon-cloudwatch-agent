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
	conf.AddJournald("JG1", "JS1", []*EventFilter{{Type: "exclude", Expression: "debug"}}, 7)
	conf.AddJournald("JG2", "JS2", []*EventFilter{{Type: "exclude", Expression: "trace|verbose"}}, 30)

	expectedKey := "journald"
	expectedVal := map[string]interface{}{
		"collect_list": []map[string]interface{}{
			{
				"log_group_name":    "JG1",
				"log_stream_name":   "JS1",
				"filters":           []map[string]interface{}{{"type": "exclude", "expression": "debug"}},
				"retention_in_days": 7,
			},
			{
				"log_group_name":    "JG2",
				"log_stream_name":   "JS2",
				"filters":           []map[string]interface{}{{"type": "exclude", "expression": "trace|verbose"}},
				"retention_in_days": 30,
			},
		},
	}

	ctx := &runtime.Context{}
	actualKey, actualVal := conf.ToMap(ctx)

	assert.Equal(t, expectedKey, actualKey)
	assert.Equal(t, expectedVal, actualVal)
}