// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/question/events"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

func TestProcessor_Process(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := new(runtime.Context)
	ctx.OsParameter = util.OsTypeLinux
	conf := new(data.Config)

	testutil.Type(inputChan, "", "", "", "2", ".*debug.*", "2", "", "", "1", "2")
	Processor.Process(ctx, conf)
	_, confMap := conf.ToMap(ctx)
	assert.Equal(t, map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"journald": map[string]interface{}{
					"collect_list": []map[string]interface{}{
						{"log_group_name": "journald", "log_stream_name": "{instance_id}", "filters": []map[string]interface{}{{"type": "exclude", "expression": ".*debug.*"}}, "retention_in_days": -1}}}}}},
		confMap)
}

func TestProcessor_NextProcessor(t *testing.T) {
	nextProcessor := Processor.NextProcessor(nil, nil)
	assert.Equal(t, events.Processor, nextProcessor)
}