// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

type DeploymentEnvironment struct {
}

func (f *DeploymentEnvironment) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, result := translator.DefaultCase("deployment.environment", "", input)

	if result == "" {
		result = agent.Global_Config.DeploymentEnvironment
	}
	// Set global log deployment environment
	GlobalLogConfig.DeploymentEnvironment = result.(string)

	return
}
