// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metric_value_benchmark

import (
	"github.com/aws/amazon-cloudwatch-agent/integration/test/metric"
	"github.com/aws/amazon-cloudwatch-agent/integration/test/status"
	"time"
)

type MemTestRunner struct {
}

var _ ITestRunner = (*MemTestRunner)(nil)

func (m *MemTestRunner) validate() status.TestGroupResult {
	metricsToFetch := m.getMeasuredMetrics()
	testResults := make([]status.TestResult, len(metricsToFetch))
	for i, name := range metricsToFetch {
		testResults[i] = m.validateMemMetric(name)
	}

	return status.TestGroupResult{
		Name:        m.getTestName(),
		TestResults: testResults,
	}
}

func (m *MemTestRunner) getTestName() string {
	return "Mem"
}

func (m *MemTestRunner) getAgentConfigFileName() string {
	return "mem_config.json"
}

func (m *MemTestRunner) getAgentRunDuration() time.Duration {
	return minimumAgentRuntime
}

func (m *MemTestRunner) getMeasuredMetrics() []string {
	return []string{
		"mem_active", "mem_available", "mem_available_percent", "mem_buffered", "mem_cached",
		"mem_free", "mem_inactive", "mem_total", "mem_used", "mem_used_percent"}
}

func (m *MemTestRunner) validateMemMetric(metricName string) status.TestResult {
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

	if !isAllValuesGreaterThanOrEqualToZero(metricName, values) {
		return testResult
	}

	testResult.Status = status.SUCCESSFUL
	return testResult
}
