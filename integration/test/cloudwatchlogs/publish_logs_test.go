// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package cloudwatchlogs

import (
	"context"
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"log"
	"strconv"

	"testing"
	"time"
)

const (
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	logScriptPath    = "resources/write_logs.sh"
	agentRunTime     = 1 * time.Minute
)

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
			test.CopyFile(param.configPath, configOutputPath)

			// give some buffer time before writing to ensure consistent
			// usage of the StartTime parameter for GetLogEvents
			time.Sleep(5 * time.Second)
			writeLogsAndRunAgent(param.iterations, agentRunTime)

			// check CWL to ensure we got the expected number of logs in the log stream
			test.ValidateLogs(t, instanceId, instanceId, param.numExpectedLogs, start)
		})
	}
}

func writeLogsAndRunAgent(iterations int, runtime time.Duration) {
	log.Printf("Writing %d of each log type\n", iterations)
	test.RunShellScript(logScriptPath, strconv.Itoa(iterations)) // write logs before starting the agent
	log.Println("Finished writing logs. Sleeping before starting agent...")
	time.Sleep(10 * time.Second)

	test.StartAgent(configOutputPath)
	time.Sleep(runtime)
	test.StopAgent()
}

func cleanUp(instanceId string) {
	test.DeleteLogGroupAndStream(instanceId, instanceId)
}
