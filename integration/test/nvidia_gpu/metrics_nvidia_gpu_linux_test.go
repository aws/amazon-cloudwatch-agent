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

func TestNvidiaGPU(t *testing.T) {
	t.Run("Basic configuration testing for both metrics and logs", func(t *testing.T) {
		test.CopyFile(configJSON, configOutputPath)
		test.StartAgent(configOutputPath, true)

		time.Sleep(agentRuntime)
		t.Logf("Agent has been running for : %s", agentRuntime.String())
		test.StopAgent()

		dimensionFilter := test.BuildDimensionFilterList(numberofAppendDimensions)
		for _, metricName := range expectedMetrics {
			test.ValidateMetrics(t, metricName, namespace, dimensionFilter)
		}

		if err := security.CheckFileRights(configOutputPath); err != nil{
			t.Fatalf("CloudWatchAgent does not have privellege to write and read CWA's log: %v",err)
		}

		if err := CheckFileOwnerRights(configOutputPath); err != nil{
			t.Fatalf("CloudWatchAgent does not have right to CWA's log: %v",err)
		}

	})
}

func CheckFileOwnerRights(filePath string) error {
	var stat syscall.Stat_t
	if err := syscall.Stat(filePath, &stat); err != nil {
		return fmt.Errorf("Cannot get file's stat %s: %v", filePath, err)
	}

	if owner, err := user.LookupId(fmt.Sprintf("%d", stat.Uid)); err != nil{
		return fmt.Errorf("Cannot look up file owner's name %s: %v", filePath, err)
	} else if owner.Name != agentPermission {
		return fmt.Errorf("Agent does not have permission to protect file %s", filePath)
	}

	return nil
}