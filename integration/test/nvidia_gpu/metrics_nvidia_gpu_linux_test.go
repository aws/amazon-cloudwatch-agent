// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metrics_nvidia_gpu

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/util/security"
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"testing"
	"time"
)

const (
	configJSON               = "resources/configLinux.json"
	namespace                = "NvidiaGPUTest"
	configOutputPath         = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	agentLogPath 			 = "/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log"
	agentRuntime             = 2 * time.Minute
	numberofAppendDimensions = 1
)

func TestNvidiaGPU(t *testing.T) {
	t.Run("Basic configuration testing for both metrics and logs", func(t *testing.T) {
		test.CopyFile(configJSON, configOutputPath)
		test.StartAgent(configOutputPath, true)

		time.Sleep(agentRuntime)
		t.Logf("Agent has been running for : %s", agentRuntime.String())
		test.StopAgent()

		dimensionFilter := test.BuildDimensionFilterList(numberofAppendDimensions)
		for _, metricName := range []string{"mem_used_percent", "utilization_gpu","utilization_memory","temperature_gpu","power_draw"} {
			test.ValidateMetrics(t, metricName, namespace, dimensionFilter)
		}

		if err := security.CheckFileRights(configOutputPath); err != nil{
			t.Fatalf("CloudWatchAgent's log does does not have privellege to write and read.")
		}

	})
}