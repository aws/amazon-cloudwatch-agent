// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metrics_nvidia_gpu

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/security"
	"os/user"
	"syscall"
	"testing"
	"time"
)

const (
	configWindowsJSON               = "resources/config_windows.json"
	metricWindowsnamespace          = "NvidiaGPUTest"
	configWindowsOutputPath         = "C:\\ProgramData\\Amazon\\AmazonCloudWatchAgent\\config.json"
	agentWindowsLogPath             = "C:\\ProgramData\\\\Amazon\\AmazonCloudWatchAgent\\Logs\\amazon-cloudwatch-agent.log"
	agentWindowsRuntime             = 2 * time.Minute
	numberofWindowsAppendDimensions = 1
)

var expectedNvidiaGPUWindowsMetrics = []string{"mem_used_percent", "nvidia_smi_utilization_gpu", "nvidia_smi_utilization_memory", "nvidia_smi_power_draw", "nvidia_smi_temperature_gpu"}

func TestNvidiaGPUWindows(t *testing.T) {
	t.Run("Run CloudWatchAgent with Nvidia-smi on Windows", func(t *testing.T) {
		err := test.CopyFile(configWindowsJSON, configWindowsOutputPath)

		if err != nil {
			t.Fatalf(Cerr)
		}

		err = test.StartAgent(configWindowsOutputPath, true)

		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(agentWindowsRuntime)
		t.Logf("Agent has been running for : %s", agentRuntime.String())
		err = test.StopAgent()

		if err != nil {
			t.Fatal("CloudWatchAgent stops failed: %v", err)
		}

		dimensionFilter := test.BuildDimensionFilterList(numberofWindowsAppendDimensions)
		for _, metricName := range expectedMetrics {
			test.ValidateMetrics(t, metricName, metricWindowsnamespace, dimensionFilter)
		}

		err = security.CheckFileRights(agentWindowsLogPath)
		if err != nil {
			t.Fatalf("CloudWatchAgent's log does not have protection from local system and admin: %v", err)
		}

	})
}
