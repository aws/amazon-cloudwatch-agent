// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metrics_number_dimension

import (
	"github.com/aws/amazon-cloudwatch-agent/integration/test/util"
	"log"
	"testing"
	"time"
)

const (
	logFilePath              = "/tmp/test.log"
	configJSON               = "resources/config.json"
	namespace                = "MetricLogsTest"
	configOutputPath         = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	metricName               = "mem_used_percent"
	agentRuntime             = 1 * time.Minute
	numberIterations         = 100
	numberExpectLogs         = 200
	numberofAppendDimensions = 1
)

func TestSimpleMetricsLogs(t *testing.T) {
	instanceId := util.GetInstanceId()
	t.Logf("Found instance id %s", instanceId)

	defer util.DeleteLogGroupAndStream(instanceId, instanceId)

	t.Run("Basic configuration testing for both metrics and logs", func(t *testing.T) {
		start := time.Now()
		util.CopyFile(configJSON, configOutputPath)
		util.StartAgent(configOutputPath, true)

		time.Sleep(agentRuntime)
		util.WriteLogs(t, logFilePath, numberIterations)
		time.Sleep(agentRuntime)
		log.Printf("Agent has been running for : %s", agentRuntime.String())
		util.StopAgent()

		dimensionFilter := util.BuildDimensionFilterList(numberofAppendDimensions)
		// check CWL to ensure we got the expected number of logs in the log stream
		util.ValidateLogs(t, instanceId, instanceId, numberExpectLogs, start)
		util.ValidateMetrics(t, metricName, namespace, dimensionFilter)
	})
}
