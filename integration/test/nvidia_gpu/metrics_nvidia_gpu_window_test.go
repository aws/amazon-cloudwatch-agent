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
	"os/user"
	"syscall"
	"fmt"
)

const (
	configJSON               = "resources/config_linux.json"
	namespace                = "NvidiaGPUTest"
	configOutputPath         = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
	agentLogPath 			 = "/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log"
	agentRuntime             = 2 * time.Minute
	agentPermission          = "root"
	numberofAppendDimensions = 1
)

var expectedMetrics = []string{"mem_used_percent","nvidia_smi_utilization_gpu","nvidia_smi_utilization_memory","nvidia_smi_power_draw","nvidia_smi_temperature_gpu"}

func TestNvidiaGPUWindows(t *testing.T) {
	t.Run("Run CloudWatchAgent with Nvidia-smi on Windows", func(t *testing.T) {
		err := test.CopyFile(configJSON, configOutputPath)
		
		if err != nil {
			t.Fatalf(Cerr)
		}
		
		err = test.StartAgent(configOutputPath, true)

		if err != nil {
			t.Fatal(err)
		}
		
		time.Sleep(agentRuntime)
		t.Logf("Agent has been running for : %s", agentRuntime.String())
		err = test.StopAgent()

		if err != nil {
			t.Fatal("CloudWatchAgent stops failed: %v",err)
		}
		
		dimensionFilter := test.BuildDimensionFilterList(numberofAppendDimensions)
		for _, metricName := range expectedMetrics {
			test.ValidateMetrics(t, metricName, namespace, dimensionFilter)
		}

		err = security.CheckFileRights(configOutputPath);
		if err != nil {
			t.Fatalf("CloudWatchAgent's log does not have protection from local system and admin: %v", err)
		}

	})
}
