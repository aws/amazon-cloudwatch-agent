// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package cloudwatchlogs

import (
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
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

// Using a single set of log group/log stream means we cannot run these tests
// in parallel without a rewrite. Publishing to the same log stream in parallel
// would mess up the count of log events that get returned in a GetLogEvents call
var (
	LogGroupName  = "cloudwatch-agent-integ-test"
	LogStreamName = "test-logs"
)

type input struct {
	iterations      int
	numExpectedLogs int
	configPath      string
}

var testParameters = []input{
	{
		iterations:      100,
		numExpectedLogs: 200,
		configPath:      "resources/config_log.json",
	},
	{
		iterations:      100,
		numExpectedLogs: 100,
		configPath:      "resources/config_log_filter.json",
	},
}

func TestWriteLogsToCloudWatch(t *testing.T) {
	cleanUp()

	for _, param := range testParameters {
		start := time.Now()
		test.CopyFile(param.configPath, configOutputPath)

		// give some buffer time before writing to ensure consistent
		// usage of the StartTime parameter for GetLogEvents
		time.Sleep(5 * time.Second)
		writeLogsAndRunAgent(param.iterations, agentRunTime)

		// check CWL to ensure we got the expected number of logs in the log stream
		test.ValidateLogs(t, LogGroupName, LogStreamName, param.numExpectedLogs, start)
	}
}

func writeLogsAndRunAgent(iterations int, runtime time.Duration) {
	log.Printf("Writing %d of each log type\n", iterations)
	test.RunShellScript(logScriptPath, strconv.Itoa(iterations)) // write logs before starting the agent
	log.Println("Finished writing logs. Sleeping before starting agent...")
	time.Sleep(5 * time.Second)

	test.StartAgent(configOutputPath)
	time.Sleep(runtime)
	test.StopAgent()
}

func cleanUp() {
	test.DeleteLogGroupAndStream(LogGroupName, LogStreamName)
}
