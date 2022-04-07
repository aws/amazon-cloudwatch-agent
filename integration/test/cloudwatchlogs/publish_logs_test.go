// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package cloudwatchlogs

import (
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"log"
	"strconv"

	"testing"
	"time"
)

const (
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	agentRunTime     = 1 * time.Minute
)

func TestWriteLogsToCloudWatch(t *testing.T) {
	start := time.Now().UnixNano() / 1e6 // convert to milliseconds
	numLogs := 100                       // number of each log type to emit

	test.CopyFile("resources/config_log.json", configOutputPath)

	// give some buffer time before writing to ensure consistent
	// usage of the StartTime parameter for GetLogEvents
	time.Sleep(1 * time.Minute)

	log.Printf("Writing %d of each log type\n", numLogs)
	test.RunShellScript("resources/write_log.sh", strconv.Itoa(numLogs)) // write logs before starting the agent

	log.Println("Finished writing logs. Sleeping before starting agent...")
	time.Sleep(5 * time.Second)
	test.StartAgent(configOutputPath)
	time.Sleep(agentRunTime)
	test.StopAgent()

	// check CWL to ensure we got the expected number of logs in the log stream
	log.Println("Getting AWS SDK client")
	c, testCtx, err := test.GetClient()
	assert.NoError(t, err)

	log.Printf("Get log events from %s/%s since %v\n", test.LogGroupName, test.LogStreamName, start)
	events, err := c.GetLogEvents(testCtx, &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  &test.LogGroupName,
		LogStreamName: &test.LogStreamName,
		StartTime:     &start,
	})

	assert.NoError(t, err)

	log.Printf("Payload: %v", events)

	assert.Len(t, events.Events, numLogs*2)
}
