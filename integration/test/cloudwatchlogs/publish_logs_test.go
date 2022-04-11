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
	"sync"

	"testing"
	"time"
)

const (
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	logLineId1       = "foo"
	logLineId2       = "bar"
	logFilePath      = "/tmp/test.log"  // TODO: not sure how well this will work on Windows
	agentRuntime     = 10 * time.Second // default flush interval is 5 seconds
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
	log.Printf("Found instance id %s", metadata.InstanceID)
	instanceId := metadata.InstanceID
	defer cleanUp(instanceId)

	for _, param := range testParameters {
		t.Run(param.testName, func(t *testing.T) {
			start := time.Now()
			var wg sync.WaitGroup

			test.CopyFile(param.configPath, configOutputPath)

			test.StartAgent(configOutputPath)

			wg.Add(1)
			go writeLogs(t, &wg, logFilePath, param.iterations)
			wg.Wait()
			time.Sleep(agentRuntime)
			test.StopAgent()

			// check CWL to ensure we got the expected number of logs in the log stream
			test.ValidateLogs(t, instanceId, instanceId, param.numExpectedLogs, start)
		})
	}
}

func writeLogs(t *testing.T, wg *sync.WaitGroup, filePath string, iterations int) {
	defer wg.Done() // always release the lock

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Error occurred creating log file for writing: %v", err)
	}
	defer f.Close()

	for i := 0; i < iterations; i++ {
		ts := time.Now()
		for _, id := range logLineIds {
			_, err = f.WriteString(fmt.Sprintf("%s - %s This is a log line.\n", ts.Format(time.RFC3339), id))
			if err != nil {
				// don't need to fatal error here. if a log line doesn't get written, the count
				// when validating the log stream should be incorrect and fail there.
				t.Logf("Error occurred writing log line: %v", err)
			}
		}
	}
}

func cleanUp(instanceId string) {
	test.DeleteLogGroupAndStream(instanceId, instanceId)
}
