// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"testing"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/question/events"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/serialization"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/testutil"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"

	"github.com/stretchr/testify/assert"
)

func TestProcessor_Process(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := new(runtime.Context)
	ctx.OsParameter = util.OsTypeLinux
	ctx.IsOnPrem = false

	conf := new(data.Config)

	testutil.Type(inputChan, "", "/var/log/messages", "", "", "", "2")
	Processor.Process(ctx, conf)
	_, confMap := conf.ToMap(ctx)
	assert.Equal(t,
		map[string]interface{}{
			"logs": map[string]interface{}{
				"logs_collected": map[string]interface{}{
					"files": map[string]interface{}{
						"collect_list": []map[string]interface{}{
							{
								"file_path":         "/var/log/messages",
								"log_group_name":    "messages",
								"log_stream_name":   "{instance_id}",
								"retention_in_days": -1,
							},
						},
					},
				},
			},
		},
		confMap)
}

func TestProcessor_NextProcessor(t *testing.T) {
	ctx := new(runtime.Context)
	conf := new(data.Config)
	nextProcessor := Processor.NextProcessor(ctx, conf)
	assert.Equal(t, serialization.Processor, nextProcessor)

	ctx.OsParameter = util.OsTypeWindows
	nextProcessor = Processor.NextProcessor(ctx, conf)
	assert.Equal(t, events.Processor, nextProcessor)
}
