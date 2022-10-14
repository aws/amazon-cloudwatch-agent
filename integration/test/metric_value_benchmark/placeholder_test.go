// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metric_value_benchmark

import (
	"time"

	"github.com/aws/amazon-cloudwatch-agent/integration/test/status"
)

const dummyTestName = "Dummy"

type DummyTestRunner struct {
}

func (t *DummyTestRunner) validate() status.TestGroupResult {
	return status.TestGroupResult{
		Name: t.getTestName(),
		TestResults: []status.TestResult{
			{
				Name:   dummyTestName,
				Status: status.SUCCESSFUL,
			},
		},
	}
}

func (t *DummyTestRunner) getTestName() string {
	return dummyTestName
}

func (t *DummyTestRunner) getAgentConfigFileName() string {
	return agentConfigFileName
}

func (t *DummyTestRunner) getAgentRunDuration() time.Duration {
	return minimumAgentRuntime
}
