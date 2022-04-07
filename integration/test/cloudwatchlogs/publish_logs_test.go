// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package cloudwatchlogs

import (
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"strconv"

	"testing"
	"time"
)

const (
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	agentRunTime     = 1 * time.Minute
)

var (
	logGroup  = "cloudwatch-agent-integ-test"
	logStream = "test-logs"
)

func TestWriteLogsToCloudWatch(t *testing.T) {
	start := time.Now().UnixNano() / 1e6 // convert to milliseconds
	numLogs := 100                       // number of each log type to emit

	test.CopyFile("resources/config_log.json", configOutputPath)

	// give some buffer time before writing to ensure consistent
	// usage of the StartTime parameter for GetLogEvents
	time.Sleep(1 * time.Minute)

	test.RunShellScript("write_log.sh", strconv.Itoa(numLogs)) // write logs before starting the agent
	test.StartAgent(configOutputPath)
	time.Sleep(agentRunTime)
	test.StopAgent()

	// check CWL to ensure we got the expected number of logs in the log stream
	c, testCtx, err := test.GetClient()
	assert.NoError(t, err)

	events, err := c.GetLogEvents(testCtx, &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  &logGroup,
		LogStreamName: &logStream,
		StartTime:     &start,
	})
	assert.NoError(t, err)

	assert.Len(t, events.Events, numLogs*2)
}
