// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metric_value_benchmark

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/amazon-cloudwatch-agent/integration/test/status"
	"github.com/stretchr/testify/suite"
)

const configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
const agentConfigDirectory = "agent_configs"
const agentConfigFileName = "/base_config.json"

const namespace = "MetricValueBenchmarkTest"
const instanceId = "InstanceId"

const minimumAgentRuntime = 3 * time.Minute

type MetricBenchmarkTestSuite struct {
	suite.Suite
	result status.TestSuiteResult
}

func (suite *MetricBenchmarkTestSuite) SetupSuite() {
	fmt.Println(">>>> Starting MetricBenchmarkTestSuite")
}

func (suite *MetricBenchmarkTestSuite) TearDownSuite() {
	suite.result.Print()
	fmt.Println(">>>> Finished MetricBenchmarkTestSuite")
}

func TestMetricValueBenchmarkSuite(t *testing.T) {
	suite.Run(t, new(MetricBenchmarkTestSuite))
}

func (suite *MetricBenchmarkTestSuite) RunAgent(agentConfigFileName string, runningDuration time.Duration) {
	agentConfigPath := agentConfigDirectory + agentConfigFileName
	log.Printf("Starting agent using agent config file %s", agentConfigPath)
	test.CopyFile(agentConfigPath, configOutputPath)
	err := test.StartAgent(configOutputPath, false)

	if err != nil {
		suite.T().Fatalf("Agent could not start")
	}

	time.Sleep(runningDuration)
	log.Printf("Agent has been running for : %s", runningDuration.String())
	test.StopAgent()
}

func isAllValuesGreaterThanZero(metricName string, values []float64) bool {
	if len(values) == 0 {
		log.Printf("No values found %v", metricName)
		return false
	}
	for _, value := range values {
		if value <= 0 {
			log.Printf("Values are not all greater than zero for %v", metricName)
			return false
		}
	}
	log.Printf("Values are all greater than zero for %v", metricName)
	return true
}
