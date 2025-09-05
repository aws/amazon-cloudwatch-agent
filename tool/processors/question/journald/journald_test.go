// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/tracesconfig"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
	"github.com/stretchr/testify/assert"
)

func TestProcessor_NextProcessor(t *testing.T) {
	ctx := &runtime.Context{}
	conf := &data.Config{}
	
	nextProcessor := Processor.NextProcessor(ctx, conf)
	assert.Equal(t, tracesconfig.Processor, nextProcessor)
}

func TestProcessor_Process_NoJournald(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := &runtime.Context{
		OsParameter: util.OsTypeLinux,
		IsOnPrem:    false,
	}
	conf := &data.Config{}

	testutil.Type(inputChan, "2") // No to journald monitoring
	Processor.Process(ctx, conf)

	logsConf := conf.LogsConf()
	assert.Nil(t, logsConf.LogsCollect)
}

func TestProcessor_Process_WithJournald(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := &runtime.Context{
		OsParameter: util.OsTypeLinux,
		IsOnPrem:    false,
	}
	conf := &data.Config{}

	testutil.Type(inputChan, "1", "1", "", "", "2", "1", "2")
	Processor.Process(ctx, conf)

	logsConf := conf.LogsConf()
	assert.NotNil(t, logsConf.LogsCollect)
	assert.NotNil(t, logsConf.LogsCollect.Journald)
	assert.Equal(t, 1, len(logsConf.LogsCollect.Journald.JournaldConfigs))
	
	journaldConfig := logsConf.LogsCollect.Journald.JournaldConfigs[0]
	assert.Equal(t, "journald", journaldConfig.LogGroup)
	assert.Equal(t, "{instance_id}", journaldConfig.LogStream)
	assert.Empty(t, journaldConfig.Units) // Empty means all units
	assert.Empty(t, journaldConfig.Filters)
	assert.Equal(t, -1, journaldConfig.RetentionInDays)
}

func TestProcessor_Process_WithSpecificUnits(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	ctx := &runtime.Context{
		OsParameter: util.OsTypeLinux,
		IsOnPrem:    true, // Test on-prem for hostname default
	}
	conf := &data.Config{}

	testutil.Type(inputChan, "1", "2", "1", "2", "1", "2", "2", "2", "2", "2", "2", "2", "my-journald-logs", "", "2", "5", "2")
	Processor.Process(ctx, conf)

	logsConf := conf.LogsConf()
	assert.NotNil(t, logsConf.LogsCollect)
	assert.NotNil(t, logsConf.LogsCollect.Journald)
	assert.Equal(t, 1, len(logsConf.LogsCollect.Journald.JournaldConfigs))
	
	journaldConfig := logsConf.LogsCollect.Journald.JournaldConfigs[0]
	assert.Equal(t, "my-journald-logs", journaldConfig.LogGroup)
	assert.Equal(t, "{hostname}", journaldConfig.LogStream) // On-prem default
	assert.Equal(t, []string{"systemd", "sshd"}, journaldConfig.Units)
	assert.Empty(t, journaldConfig.Filters)
	assert.Equal(t, 7, journaldConfig.RetentionInDays)
}