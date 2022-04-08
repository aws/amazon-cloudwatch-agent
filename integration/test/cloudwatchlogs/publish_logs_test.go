// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package cloudwatchlogs

import (
	"context"
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/aws-sdk-go-v2/config"
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

var (
	LogGroupName  = "cloudwatch-agent-integ-test"
	LogStreamName = "test-logs"
)

func TestWriteLogsToCloudWatch(t *testing.T) {
	start := time.Now().UnixNano() / 1e6 // convert to milliseconds
	numLogs := 100                       // number of each log type to emit

	test.CopyFile("resources/config_log.json", configOutputPath)

	// give some buffer time before writing to ensure consistent
	// usage of the StartTime parameter for GetLogEvents
	time.Sleep(1 * time.Minute)

	log.Printf("Writing %d of each log type\n", numLogs)
	test.RunShellScript("resources/write_logs.sh", strconv.Itoa(numLogs)) // write logs before starting the agent

	log.Println("Finished writing logs. Sleeping before starting agent...")
	time.Sleep(5 * time.Second)
	test.StartAgent(configOutputPath)
	time.Sleep(agentRunTime)
	test.StopAgent()

	// check CWL to ensure we got the expected number of logs in the log stream
	ctx := context.Background()
	c, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		t.Fatalf("An error occurred loading the SDK config: %v", err.Error())
	}

	cwl := cloudwatchlogs.NewFromConfig(c)

	log.Printf("Get log events from %s/%s since %v\n", LogGroupName, LogStreamName, start)
	events, err := cwl.GetLogEvents(ctx, &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  &LogGroupName,
		LogStreamName: &LogStreamName,
		StartTime:     &start,
	})
	if err != nil {
		t.Fatalf("An error occurred getting logs from CWL: %v", err.Error())
	}

	log.Printf("Payload: %v", events)

	assert.Len(t, events.Events, numLogs*2)
}
