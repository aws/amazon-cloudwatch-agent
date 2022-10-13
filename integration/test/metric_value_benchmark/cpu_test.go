// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metric_value_benchmark

import (
	"github.com/aws/amazon-cloudwatch-agent/integration/test/metric"
	"github.com/aws/amazon-cloudwatch-agent/integration/test/status"
	"log"
)

func (suite *MetricBenchmarkTestSuite) TestCPUValues() {
	log.Printf("Testing Cpu values...")
	suite.RunAgent(agentConfigFileName, minimumAgentRuntime)
	testGroupResult := validateCpuMetrics()
	suite.addToSuiteResult(testGroupResult)
	suite.Assert().Equal(status.SUCCESSFUL, testGroupResult.GetStatus(),
		"Cpu test failed to validate that every metric value is greater than zero")

	// TODO: Range test with >0 and <100
	// TODO: Range test: which metric to get? api reference check. should I get average or test every single datapoint for 10 minutes? (and if 90%> of them are in range, we are good)
}

func (suite *MetricBenchmarkTestSuite) addToSuiteResult(r status.TestGroupResult) {
	suite.result.TestGroupResults = append(suite.result.TestGroupResults, r)
}

var metricsToFetch = []string{
	"cpu_time_active", "cpu_time_guest", "cpu_time_guest_nice", "cpu_time_idle", "cpu_time_iowait", "cpu_time_irq",
	"cpu_time_nice", "cpu_time_softirq", "cpu_time_steal", "cpu_time_system", "cpu_time_user",
	"cpu_usage_active", "cpu_usage_quest", "cpu_usage_quest_nice", "cpu_usage_idle", "cpu_usage_iowait",
	"cpu_usage_irq", "cpu_usage_nice", "cpu_usage_softirq", "cpu_usage_steal", "cpu_usage_system", "cpu_usage_user"}

func validateCpuMetrics() status.TestGroupResult {
	testResults := []status.TestResult{}
	for _, metricName := range metricsToFetch {
		testResult := validateCpuMetric(metricName)
		testResults = append(testResults, testResult)
	}

	return status.TestGroupResult{
		Name:        "CPU",
		TestResults: testResults,
	}
}

func validateCpuMetric(metricName string) status.TestResult {
	testResult := status.TestResult{
		Name:   metricName,
		Status: status.FAILED,
	}

	fetcher, err := metric.GetMetricFetcher(metricName)
	if err != nil {
		return testResult
	}

	values, err := fetcher.Fetch(namespace, metricName, metric.AVERAGE)
	if err != nil {
		return testResult
	}

	if !isAllValuesGreaterThanZero(metricName, values) {
		return testResult
	}

	testResult.Status = status.SUCCESSFUL
	return testResult
}
