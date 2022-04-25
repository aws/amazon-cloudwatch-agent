// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package cloudwatchlogs

import (
	"context"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"log"
	"os"
	"strings"

	"testing"
	"time"
)

const (
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	logLineId1       = "foo"
	logLineId2       = "bar"
	logFilePath      = "/tmp/test.log"  // TODO: not sure how well this will work on Windows
	agentRuntime     = 20 * time.Second // default flush interval is 5 seconds
)

var logLineIds = []string{logLineId1, logLineId2}

type input struct {
	testName        string
	iterations      int
	numExpectedLogs int
	configPath      string
}

var testParameters = []input{
	{
		testName:        "Happy path",
		iterations:      100,
		numExpectedLogs: 200,
		configPath:      "resources/config_log.json",
	},
	{
		testName:        "Client-side log filtering",
		iterations:      100,
		numExpectedLogs: 100,
		configPath:      "resources/config_log_filter.json",
	},
}

func TestWriteLogsToCloudWatch(t *testing.T) {
	// this uses the {instance_id} placeholder in the agent configuration,
	// so we need to determine the host's instance ID for validation
	ctx := context.Background()
	c, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		// fail fast so we don't continue the test
		t.Fatalf("Error occurred while creating SDK config: %v", err)
	}

	// TODO: this only works for EC2 based testing
	client := imds.NewFromConfig(c)
	metadata, err := client.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		t.Fatalf("Error occurred while retrieving EC2 instance ID: %v", err)
	}
	instanceId := metadata.InstanceID
	log.Printf("Found instance id %s", instanceId)

	defer cleanUp(instanceId)

	for _, param := range testParameters {
		t.Run(param.testName, func(t *testing.T) {
			start := time.Now()

			test.CopyFile(param.configPath, configOutputPath)

			test.StartAgent(configOutputPath)

			// ensure that there is enough time from the "start" time and the first log line,
			// so we don't miss it in the GetLogEvents call
			time.Sleep(agentRuntime)
			writeLogs(t, logFilePath, param.iterations)
			time.Sleep(agentRuntime)
			test.StopAgent()

			// check CWL to ensure we got the expected number of logs in the log stream
			test.ValidateLogs(t, instanceId, instanceId, param.numExpectedLogs, start)
		})
	}
}

// Validate https://github.com/aws/amazon-cloudwatch-agent/issues/447
func TestRotatingLogsDoesNotSkipLines(t *testing.T) {
	cfgFilePath := "resources/config_log_rotated.json"
	line1 := strings.Repeat("12345", 5)
	line2 := strings.Repeat("09876", 5)
	line3 := strings.Repeat("1234567890", 5)
	lines := []string{line1, line2, line3}

	// this uses the {instance_id} placeholder in the agent configuration,
	// so we need to determine the host's instance ID for validation
	ctx := context.Background()
	c, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		// fail fast so we don't continue the test
		t.Fatalf("Error occurred while creating SDK config: %v", err)
	}

	// TODO: this only works for EC2 based testing
	client := imds.NewFromConfig(c)
	metadata, err := client.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		t.Fatalf("Error occurred while retrieving EC2 instance ID: %v", err)
	}
	instanceId := metadata.InstanceID
	log.Printf("Found instance id %s", instanceId)

	defer cleanUp(instanceId)

	start := time.Now()
	test.CopyFile(cfgFilePath, configOutputPath)

	test.StartAgent(configOutputPath)

	// ensure that there is enough time from the "start" time and the first log line,
	// so we don't miss it in the GetLogEvents call
	time.Sleep(agentRuntime)
	truncateAndWriteLogs(t, logFilePath, lines)
	time.Sleep(agentRuntime)
	test.StopAgent()

	t.Log(test.ReadAgentOutput(1 * time.Minute))

	test.ValidateLogsInOrder(t, instanceId, instanceId+"Rotated", lines, start)
}

func writeLogs(t *testing.T, filePath string, iterations int) {
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Error occurred creating log file for writing: %v", err)
	}
	defer f.Close()

	log.Printf("Writing %d lines to %s", iterations*len(logLineIds), filePath)

	for i := 0; i < iterations; i++ {
		ts := time.Now()
		for _, id := range logLineIds {
			_, err = f.WriteString(fmt.Sprintf("%s - [%s] #%d This is a log line.\n", ts.Format(time.StampMilli), id, i))
			if err != nil {
				// don't need to fatal error here. if a log line doesn't get written, the count
				// when validating the log stream should be incorrect and fail there.
				t.Logf("Error occurred writing log line: %v", err)
			}
		}
		time.Sleep(1 * time.Millisecond)
	}
}

func truncateAndWriteLogs(t *testing.T, filePath string, lines []string) {
	log.Printf("Writing %d lines to %s", len(lines), filePath)

	for _, logLine := range lines {
		_ = os.Remove(filePath) // try to remove the file regardless of whether it exists or not
		time.Sleep(1 * time.Second)
		f, err := os.Create(filePath)
		if err != nil {
			t.Fatalf("Error occurred creating log file for writing: %v", err)
		}
		_, err = f.WriteString(logLine + "\n")
		if err != nil {
			t.Fatalf("Error occurred when writing %s to %s: %v", logLine, filePath, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func cleanUp(instanceId string) {
	test.DeleteLogGroupAndStream(instanceId, instanceId)
}
