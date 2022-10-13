// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metric_value_benchmark

import (
	"fmt"
	"log"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/amazon-cloudwatch-agent/integration/test/metric"
	"github.com/aws/amazon-cloudwatch-agent/integration/test/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
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
	// TODO: Print all results from suite.testResults

	// TODO: assert here so the entire suite fails ? or can each test fail and suite handles it automatically without skipping?

	fmt.Println(">>>> Finished MetricBenchmarkTestSuite")
}

func TestMetricValueBenchmarkSuite(t *testing.T) {
	suite.Run(t, new(MetricBenchmarkTestSuite))
}

// TODO assert instead of if statement

// TODO each test function in separate files

const agentConfigDirectory = "agent_configs"

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

func (suite *MetricBenchmarkTestSuite) TestDummy() {
	assert.Equal(suite.T(), true, false,
		"Always fail")
}

func (suite *MetricBenchmarkTestSuite) TestCPUValues() {
	log.Printf("Testing Cpu values...")
	suite.RunAgent(agentConfigFileName, minimumAgentRuntime)

	testResult := validateCpuMetrics()
	testSuiteStatus := getTestSuiteStatus(testResult)
	printTestResult(testSuiteStatus, testResult)

	assert.Equal(suite.T(), status.SUCCESSFUL, testSuiteStatus,
		"Cpu test failed to validate that every metric value is greater than zero")

	// TODO: Get CPU value > 0
	// TODO: Range test with >0 and <100
	// TODO: Range test: which metric to get? api reference check. should I get average or test every single datapoint for 10 minutes? (and if 90%> of them are in range, we are good)
}

var metricsToFetch = []string{
	"cpu_time_active", "cpu_time_guest", "cpu_time_guest_nice", "cpu_time_idle", "cpu_time_iowait", "cpu_time_irq",
	"cpu_time_nice", "cpu_time_softirq", "cpu_time_steal", "cpu_time_system", "cpu_time_user",
	"cpu_usage_active", "cpu_usage_quest", "cpu_usage_quest_nice", "cpu_usage_idle", "cpu_usage_iowait",
	"cpu_usage_irq", "cpu_usage_nice", "cpu_usage_softirq", "cpu_usage_steal", "cpu_usage_system", "cpu_usage_user"}

func validateCpuMetrics() map[string]status.TestStatus {
	validationResult := map[string]status.TestStatus{}
	for _, metricName := range metricsToFetch {
		validationResult[metricName] = status.FAILED

		fetcher, err := metric.GetMetricFetcher(metricName)
		if err != nil {
			continue
		}

		values, err := fetcher.Fetch(namespace, metricName, metric.AVERAGE)
		if err != nil {
			continue
		}

		if !isAllValuesGreaterThanZero(metricName, values) {
			continue
		}

		validationResult[metricName] = status.SUCCESSFUL
	}
	return validationResult
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

func printTestResult(testSuiteStatus status.TestStatus, testSummary map[string]status.TestStatus) {
	testSuite := "CPU Test"

	log.Printf("Finished %v", testSuite)
	log.Printf("==============%v==============", testSuite)
	log.Printf("==============%v==============", string(testSuiteStatus))
	w := tabwriter.NewWriter(log.Writer(), 1, 1, 1, ' ', 0)
	for metricName, status := range testSummary {
		fmt.Fprintln(w, metricName, "\t", status, "\t")
	}
	w.Flush()
	log.Printf("==============================")
}

func getTestSuiteStatus(testSummary map[string]status.TestStatus) status.TestStatus {
	for _, value := range testSummary {
		if value == status.FAILED {
			return status.FAILED
		}
	}
	return status.SUCCESSFUL
}
