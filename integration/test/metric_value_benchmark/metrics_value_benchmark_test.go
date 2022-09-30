// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metric_value_benchmark

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/amazon-cloudwatch-agent/integration/test/metric"
	"log"
	"testing"
	"time"
)

const configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
const configJSON = "/base_config.json"

//const namespace = "MetricValueBenchmarkTest"
const instanceId = "InstanceId"

//Let the agent run for 2 minutes. This will give agent enough time to call server
const agentRuntime = 3 * time.Minute

func TestCPUValue(t *testing.T) {
	log.Printf("testing cpu value...")

	resourcePath := "agent_configs"

	log.Printf("resource file location %s", resourcePath)

	t.Run(fmt.Sprintf("resource file location %s ", resourcePath), func(t *testing.T) {
		test.CopyFile(resourcePath+configJSON, configOutputPath)
		err := test.StartAgent(configOutputPath, false)

		if err != nil {
			t.Fatalf("Agent could not start")
		}

		time.Sleep(agentRuntime)
		log.Printf("Agent has been running for : %s", agentRuntime.String())
		test.StopAgent()

		err = metric.FetchMetricValues()
		if err != nil {
			t.Fatalf("Error while fetching metric value: %v", err.Error())
		}

		log.Printf("Finished Testing CPU Value")
		/*
			// test for cloud watch metrics
			dimensionFilter := buildDimensionFilterList(parameter.numberDimensionsInCW)
			test.ValidateMetrics(t, parameter.metricName, namespace, dimensionFilter) */
	})

	// TODO: Get CPU value > 0
	// TODO: Range test with >0 and <100
	// TODO: Range test: which metric to get? api reference check. should I get average or test every single datapoint for 10 minutes? (and if 90%> of them are in range, we are good)
}
