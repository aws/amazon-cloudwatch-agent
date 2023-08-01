// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package linux

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/data/config"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/processors/question/logs"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/runtime"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/testutil"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/tool/util"
)

func TestProcessor_Process(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()
	ctx := new(runtime.Context)
	conf := new(data.Config)

	tomlString := `
	[general]
		state_file = /var/lib/awslogs/agent-state

	[/var/log/messages]
		datetime_format = %b %d %H:%M:%S
		file = /var/log/messages
		buffer_duration = 5000
		log_stream_name = {hostname}
		initial_position = start_of_file
		log_group_name = /var/log/messages
		log_group_class = standard
	`
	tmpFile, _ := os.CreateTemp("", "")
	defer os.Remove(tmpFile.Name())

	err := os.WriteFile(tmpFile.Name(), []byte(tomlString), os.ModePerm)
	assert.NoError(t, err)

	expectedMap := map[string]interface{}{
		"logs": map[string]interface{}{
			"force_flush_interval": 5,
			"logs_collected": map[string]interface{}{
				"files": map[string]interface{}{
					"collect_list": []map[string]interface{}{
						{
							"timestamp_format":  "%b %d %H:%M:%S",
							"file_path":         "/var/log/messages",
							"log_group_name":    "/var/log/messages",
							"log_stream_name":   "{hostname}",
							"log_group_class":   "standard",
							"retention_in_days": -1,
						},
					},
				},
			},
		},
	}

	testutil.Type(inputChan, "1", tmpFile.Name())

	Processor.Process(ctx, conf)
	_, resultMap := conf.ToMap(ctx)
	assert.Equal(t, expectedMap, resultMap)
}

func TestProcessor_NextProcessor(t *testing.T) {
	assert.Equal(t, logs.Processor, Processor.NextProcessor(nil, nil))
}

func TestAnyExistingLogAgentConfigFileToImport(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "", "1")
	assert.Equal(t, false, util.No(anyExistingLinuxConfigQuestion))
	assert.Equal(t, true, util.No(anyExistingLinuxConfigQuestion))
}

func TestFilePathForTheExistingConfigFile(t *testing.T) {
	inputChan := testutil.SetUpTestInputStream()

	testutil.Type(inputChan, "", "/var/test.conf")
	assert.Equal(t, "/var/awslogs/etc/awslogs.conf", util.AskWithDefault(filePathLinuxConfigQuestion, DefaultFilePathLinuxConfiguration))
	assert.Equal(t, "/var/test.conf", util.AskWithDefault(filePathLinuxConfigQuestion, DefaultFilePathLinuxConfiguration))
}

func TestProcessConfigFromPythonConfigParserFile(t *testing.T) {
	tomlString := `
	[general]
		state_file = /var/lib/awslogs/agent-state

	[/var/output/log/audit_pusher.log]
		file = /var/output/log/audit_pusher.log
		log_group_name = service/audit_pusher
		log_stream_name = hsm-bqvuwqn72vk
		datetime_format = %b %d %H:%M:%S,%f
		encoding = euc_jp
		buffer_duration = 5000

	[/var/log/messages]
		datetime_format = %b %d %H:%M:%S
		file = /var/log/messages
		buffer_duration = 5000
		log_stream_name = {hostname}
		initial_position = start_of_file
		log_group_name = /var/log/messages
		multi_line_start_pattern = {datetime_format}
	`
	expectedMap := map[string]interface{}{
		"force_flush_interval": 5,
		"logs_collected": map[string]interface{}{
			"files": map[string]interface{}{
				"collect_list": []map[string]interface{}{
					{
						"file_path":                "/var/log/messages",
						"log_group_name":           "/var/log/messages",
						"timestamp_format":         "%b %d %H:%M:%S",
						"multi_line_start_pattern": "{timestamp_format}",
						"log_stream_name":          "{hostname}",
						"retention_in_days":        -1,
					},
					{
						"file_path":         "/var/output/log/audit_pusher.log",
						"log_group_name":    "service/audit_pusher",
						"timestamp_format":  "%b %d %H:%M:%S,%f",
						"log_stream_name":   "hsm-bqvuwqn72vk",
						"encoding":          "euc-jp",
						"retention_in_days": -1,
					},
				},
			},
		},
	}

	tmpFile, _ := os.CreateTemp("", "")
	defer os.Remove(tmpFile.Name())

	err := os.WriteFile(tmpFile.Name(), []byte(tomlString), os.ModePerm)
	assert.NoError(t, err)

	ctx := new(runtime.Context)
	ctx.OsParameter = util.OsTypeLinux
	logsConfig := new(config.Logs)
	processConfigFromPythonConfigParserFile(tmpFile.Name(), logsConfig)
	_, resultMap := logsConfig.ToMap(ctx)
	assert.Equal(t, expectedMap, resultMap)
}
