// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metrics_number_dimension

import (
	"log"
	"testing"
	"time"
	"github.com/aws/amazon-cloudwatch-agent/integration/test/utils"
)

const (
	instanceId       = "InstanceId"
	configJSON       = "resources/config.json"
	namespace        = "MetricLogsSimpleTesting3232"
	configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	metricName       = "mem_used_percent"
	agentRuntime     = 2 * time.Minute
	numberExpectLogs = 200
)

func TestSimpleMetricsLogs(t *testing.T) {
	t.Run("Basic configuration testing for both metrics and logs", func(t *testing.T) {
		start := time.Now()
		utils.CopyFile(configJSON, configOutputPath)
		err := utils.StartAgent(configOutputPath, false)

		utils.WriteLogs(t, logFilePath, param.iterations)
		time.Sleep(agentRuntime)
		log.Printf("Agent has been running for : %s", agentRuntime.String())
		utils.StopAgent()

		// check CWL to ensure we got the expected number of logs in the log stream
		test.ValidateLogs(t, instanceId, instanceId, numberExpectLogs, start)
		test.ValidateMetrics(t, metricName, namespace)
	})

}
