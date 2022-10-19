// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metric_value_benchmark

import (
	"time"

	"github.com/aws/amazon-cloudwatch-agent/integration/test/status"
)

type DummyTestRunner struct {
}

var _ ITestRunner = (*DummyTestRunner)(nil)

func (t *DummyTestRunner) validate() status.TestGroupResult {
	return status.TestGroupResult{
		Name: t.getTestName(),
		TestResults: []status.TestResult{
			{
				Name:   t.getTestName(),
				Status: status.SUCCESSFUL,
			},
		},
	}
}

func (t *DummyTestRunner) getTestName() string {
	return "Dummy"
}

func (t *DummyTestRunner) getAgentConfigFileName() string {
	return "base_linux_config.json" // default configuration
}

func (t *DummyTestRunner) getAgentRunDuration() time.Duration {
	return minimumAgentRuntime
}

func (t *DummyTestRunner) getMeasuredMetrics() []string {
	return []string{}
}
