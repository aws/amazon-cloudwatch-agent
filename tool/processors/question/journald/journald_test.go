// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/tracesconfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

func TestProcessor_NextProcessor(t *testing.T) {
	nextProcessor := Processor.NextProcessor(nil, nil)
	assert.Equal(t, tracesconfig.Processor, nextProcessor)
}

func TestProcessor_Process_NoJournald(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()
	ctx := &runtime.Context{OsParameter: util.OsTypeLinux}
	conf := new(data.Config)

	testutil.Type(inputChan, "2")
	Processor.Process(ctx, conf)

	assert.Nil(t, conf.LogsConf().LogsCollect)
}

func TestProcessor_Process_WithAllUnits(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()
	ctx := &runtime.Context{OsParameter: util.OsTypeLinux}
	conf := new(data.Config)

	testutil.Type(inputChan, "1", "", "", "2", "2", "2", "2", "1", "2")
	Processor.Process(ctx, conf)

	journaldConfig := conf.LogsConf().LogsCollect.Journald.JournaldConfigs[0]
	assert.Equal(t, "journald", journaldConfig.LogGroup)
	assert.Equal(t, "{instance_id}", journaldConfig.LogStream)
	assert.Empty(t, journaldConfig.Units)
	assert.Empty(t, journaldConfig.Priority)
	assert.Nil(t, journaldConfig.Matches)
	assert.Empty(t, journaldConfig.Filters)
	assert.Equal(t, -1, journaldConfig.RetentionInDays)
}

func TestProcessor_Process_WithSpecificUnits(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()
	ctx := &runtime.Context{OsParameter: util.OsTypeLinux, IsOnPrem: true}
	conf := new(data.Config)

	testutil.Type(inputChan, "1", "my-logs", "", "1", "sshd", "2", "2", "2", "5", "2")
	Processor.Process(ctx, conf)

	journaldConfig := conf.LogsConf().LogsCollect.Journald.JournaldConfigs[0]
	assert.Equal(t, "my-logs", journaldConfig.LogGroup)
	assert.Equal(t, "{hostname}", journaldConfig.LogStream)
	assert.Equal(t, []string{"sshd"}, journaldConfig.Units)
	assert.Equal(t, 7, journaldConfig.RetentionInDays)
}

func TestProcessor_Process_WithAllFields(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()
	ctx := &runtime.Context{OsParameter: util.OsTypeLinux}
	conf := new(data.Config)

	testutil.Type(inputChan, "1", "all-logs", "my-stream", "1", "sshd", "1", "4", "1", "_UID", "0", "2", "1", "1", ".*error.*", "2", "5", "2")
	Processor.Process(ctx, conf)

	journaldConfig := conf.LogsConf().LogsCollect.Journald.JournaldConfigs[0]
	assert.Equal(t, "all-logs", journaldConfig.LogGroup)
	assert.Equal(t, "my-stream", journaldConfig.LogStream)
	assert.Equal(t, []string{"sshd"}, journaldConfig.Units)
	assert.Equal(t, "err", journaldConfig.Priority)
	assert.Equal(t, []map[string]string{{"_UID": "0"}}, journaldConfig.Matches)
	assert.Equal(t, 1, len(journaldConfig.Filters))
	assert.Equal(t, "include", journaldConfig.Filters[0].Type)
	assert.Equal(t, ".*error.*", journaldConfig.Filters[0].Expression)
	assert.Equal(t, 7, journaldConfig.RetentionInDays)
}
