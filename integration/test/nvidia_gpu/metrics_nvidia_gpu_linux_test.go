// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux, integration

package metrics_nvidia_gpu

import (
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"testing"
	"time"
	"strings"
)

const (
	configJSON               = "resources/configLinux.json"
	namespace                = "NvidiaGPUTest"
	configOutputPath         = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	agentLogPath 			 = "/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log"
	agentRuntime             = 2 * time.Minute
	numberofAppendDimensions = 1
)

func TestNumberMetricDimensions(t *testing.T) {
	t.Run("Basic configuration testing for both metrics and logs", func(t *testing.T) {
		test.CopyFile(configJSON, configOutputPath)
		test.StartAgent(configOutputPath, true)

		time.Sleep(agentRuntime)
		t.Logf("Agent has been running for : %s", agentRuntime.String())
		test.StopAgent()

		dimensionFilter := test.BuildDimensionFilterList(numberofAppendDimensions)
		for _, metricName := range []string{"mem_used_percent", "utilization_gpu","utilization_memory","temperature_gpu","power_draw"} {
			util.ValidateMetrics(t, metricName, namespace, dimensionFilter)
		}

		ValidateFilePermission(t)

	})
}

func ValidateFilePermission(t *testing.T) {
	
	if ownerPermission, fileUserOwner, fileGroupOwner := util.CheckFilePermissionAndOwner(configOutputPath,"owner");
			!strings.Contains(ownerPermission,"r") || !strings.Contains(ownerPermission,"w") || fileUserOwner != "root" || fileGroupOwner != "root" {
		t.Fatalf("CloudWatchAgent's log does does not have privellege to write and read.")
	}

	if othersPermission, _, _ := util.CheckFilePermissionAndOwner(agentLogPath,"others");
			strings.Contains(ownerPermission,"w") || strings.Contains(ownerPermission,"x")  {
		t.Fatalf("Others have more than read permission.")
	}
}