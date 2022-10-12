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
)

const configOutputPath = "/opt/aws/amazon-cloudwatch-agent/bin/config.json"
const configJSON = "/base_config.json"

const namespace = "MetricValueBenchmarkTest"
const instanceId = "InstanceId"

const minimumAgentRuntime = 3 * time.Minute

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

		time.Sleep(minimumAgentRuntime)
		log.Printf("Agent has been running for : %s", minimumAgentRuntime.String())
		test.StopAgent()

		testResult := validateCpuMetrics()
		testSuiteStatus := getTestSuiteStatus(testResult)
		printTestResult(testSuiteStatus, testResult)

		if testSuiteStatus == status.FAILED {
			t.Fatalf("Cpu test failed to validate that every metric value is greater than zero")
		}
	})

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
	isAllSuccessful := status.SUCCESSFUL
	for _, value := range testSummary {
		if value == status.FAILED {
			isAllSuccessful = status.FAILED
			break
		}
	}
	return isAllSuccessful
}
