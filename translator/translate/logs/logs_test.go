// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

func TestLogs(t *testing.T) {
	l := new(Logs)
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.RegionType = "any"

	var input interface{}
	err := json.Unmarshal([]byte(`{"logs":{"log_stream_name":"LOG_STREAM_NAME"}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	_, actual := l.ApplyRule(input)
	expected := map[string]interface{}{
		"outputs": map[string]interface{}{
			"cloudwatchlogs": []interface{}{
				map[string]interface{}{
					"region":               "us-east-1",
					"region_type":          "any",
					"mode":                 "",
					"log_stream_name":      "LOG_STREAM_NAME",
					"force_flush_interval": "5s",
				},
			},
		},
	}
	assert.Equal(t, expected, actual, "Expected to be equal")
}

func TestLogs_LogStreamName(t *testing.T) {
	l := new(Logs)
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.RegionType = "any"

	var input interface{}
	err := json.Unmarshal([]byte(`{"logs":{}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	ctx := context.CurrentContext()
	ctx.SetMode(config.ModeOnPrem)

	hostname, _ := os.Hostname()
	_, actual := l.ApplyRule(input)
	expected := map[string]interface{}{
		"outputs": map[string]interface{}{
			"cloudwatchlogs": []interface{}{
				map[string]interface{}{
					"region":               "us-east-1",
					"region_type":          "any",
					"mode":                 "OP",
					"log_stream_name":      hostname,
					"force_flush_interval": "5s",
				},
			},
		},
	}

	assert.Equal(t, expected, actual, "Expected to be equal")

	context.ResetContext()

	// default log stream name from task arn
	ctx = context.CurrentContext()
	ctx.SetRunInContainer(true)
	ecsUtilInstance := ecsutil.GetECSUtilSingleton()
	ecsUtilInstance.Region = "us-east-1"
	ecsUtilInstance.Cluster = "cluster-name"
	ecsUtilInstance.TaskARN = "arn:aws:ecs:us-east-2:012345678910:task/cluster-name/9781c248-0edd-4cdb-9a93-f63cb662a5d3"

	_, actual = l.ApplyRule(input)
	expected = map[string]interface{}{
		"outputs": map[string]interface{}{
			"cloudwatchlogs": []interface{}{
				map[string]interface{}{
					"region":               "us-east-1",
					"region_type":          "any",
					"mode":                 "",
					"log_stream_name":      "arn_aws_ecs_us-east-2_012345678910_task/cluster-name/9781c248-0edd-4cdb-9a93-f63cb662a5d3",
					"force_flush_interval": "5s",
				},
			},
		},
	}
	assert.Equal(t, expected, actual, "Expected to be equal")

	context.ResetContext()
	ecsUtilInstance.Region = ""

	// default log stream name from pod id env variable
	ctx = context.CurrentContext()
	ctx.SetRunInContainer(true)
	os.Setenv(config.POD_NAME, "demo-app-5ffc89b95c-jgnf6")

	_, actual = l.ApplyRule(input)
	expected = map[string]interface{}{
		"outputs": map[string]interface{}{
			"cloudwatchlogs": []interface{}{
				map[string]interface{}{
					"region":               "us-east-1",
					"region_type":          "any",
					"mode":                 "",
					"log_stream_name":      "demo-app-5ffc89b95c-jgnf6",
					"force_flush_interval": "5s",
				},
			},
		},
	}
	assert.Equal(t, expected, actual, "Expected to be equal")

	os.Clearenv()
	context.ResetContext()
}

func TestLogs_ForceFlushInterval(t *testing.T) {
	l := new(Logs)
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.RegionType = "any"

	var input interface{}
	err := json.Unmarshal([]byte(`{"logs":{"force_flush_interval":10}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	ctx := context.CurrentContext()
	ctx.SetMode(config.ModeOnPrem)

	hostname, _ := os.Hostname()
	_, actual := l.ApplyRule(input)
	expected := map[string]interface{}{
		"outputs": map[string]interface{}{
			"cloudwatchlogs": []interface{}{
				map[string]interface{}{
					"region":               "us-east-1",
					"region_type":          "any",
					"mode":                 "OP",
					"log_stream_name":      hostname,
					"force_flush_interval": "10s",
				},
			},
		},
	}

	assert.Equal(t, expected, actual, "Expected to be equal")

	ctx.SetMode(config.ModeEC2) //reset back to default mode
}

func TestLogs_EndpointOverride(t *testing.T) {
	l := new(Logs)
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.RegionType = "any"

	var input interface{}
	err := json.Unmarshal([]byte(`{"logs":{"endpoint_override":"https://logs-fips.us-east-1.amazonaws.com"}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	ctx := context.CurrentContext()
	ctx.SetMode(config.ModeOnPrem)

	hostname, _ := os.Hostname()
	_, actual := l.ApplyRule(input)
	expected := map[string]interface{}{
		"outputs": map[string]interface{}{
			"cloudwatchlogs": []interface{}{
				map[string]interface{}{
					"region":               "us-east-1",
					"region_type":          "any",
					"mode":                 "OP",
					"endpoint_override":    "https://logs-fips.us-east-1.amazonaws.com",
					"log_stream_name":      hostname,
					"force_flush_interval": "5s",
				},
			},
		},
	}

	assert.Equal(t, expected, actual, "Expected to be equal")

	ctx.SetMode(config.ModeEC2) //reset back to default mode
}

func TestLogs_ServiceAndEnvironment(t *testing.T) {
	l := new(Logs)
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.RegionType = "any"

	context.ResetContext()

	var input interface{}
	err := json.Unmarshal([]byte(`{"logs":{"service.name": "my-service",
     "deployment.environment": "ec2:group","log_stream_name":"LOG_STREAM_NAME"}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	_, _ = l.ApplyRule(input)
	assert.Equal(t, "my-service", GlobalLogConfig.ServiceName)
	assert.Equal(t, "ec2:group", GlobalLogConfig.DeploymentEnvironment)
}

func TestLogs_ServiceAndEnvironmentMissing(t *testing.T) {
	l := new(Logs)
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.RegionType = "any"
	agent.Global_Config.DeploymentEnvironment = "ec2:group"
	agent.Global_Config.ServiceName = "my-service"

	context.ResetContext()

	var input interface{}
	err := json.Unmarshal([]byte(`{"logs":{"log_stream_name":"LOG_STREAM_NAME"}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	_, _ = l.ApplyRule(input)
	assert.Equal(t, "my-service", GlobalLogConfig.ServiceName)
	assert.Equal(t, "ec2:group", GlobalLogConfig.DeploymentEnvironment)
}

func TestLogs_Concurrency(t *testing.T) {
	l := new(Logs)
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.RegionType = "any"
	context.ResetContext()

	var input interface{}
	err := json.Unmarshal([]byte(`{"logs":{"concurrency":10}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	_, _ = l.ApplyRule(input)

	assert.Equal(t, 10, GlobalLogConfig.Concurrency)
}

func TestLogs_Concurrency_Default(t *testing.T) {
	l := new(Logs)
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.RegionType = "any"
	context.ResetContext()
	context.CurrentContext().SetAgentLogFile("/tmp/amazon-cloudwatch-agent.log")

	testCases := map[string]struct {
		input map[string]any
		want  int
	}{
		"WithMissingLogsCollected": {
			input: map[string]any{
				"logs": map[string]any{},
			},
			want: -1,
		},
		"WithLogFileOnly": {
			input: testutil.GetJson(t, filepath.Join("testdata", "default_concurrency", "logfile_only.json")),
			want:  defaultConcurrency,
		},
		"WithWindowsEventsOnly": {
			input: testutil.GetJson(t, filepath.Join("testdata", "default_concurrency", "windows_events_only.json")),
			want:  defaultConcurrency,
		},
		"WithMissingTimestampFormat/LogFile": {
			input: testutil.GetJson(t, filepath.Join("testdata", "default_concurrency", "missing_timestamp.json")),
			want:  -1,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			_, _ = l.ApplyRule(testCase.input)

			assert.Equal(t, testCase.want, GlobalLogConfig.Concurrency)
		})
	}
}
