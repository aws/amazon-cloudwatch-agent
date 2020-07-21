// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package providers

import (
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/awscsm/metametrics"
)

type MockConfigProvider struct{}

func (c *MockConfigProvider) RetrieveAgentConfig() AgentConfig {
	return AgentConfig{
		Status:      "Ready",
		Limits:      defaultLimits,
		Definitions: DefaultDefinitions(),
	}
}

func (c *MockConfigProvider) Close() {}

func (c *MockConfigProvider) Write(metrics metametrics.Metrics) error {
	return nil
}
