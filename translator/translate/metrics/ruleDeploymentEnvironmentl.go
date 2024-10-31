// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

type DeploymentEnvironment struct {
}

func (f *DeploymentEnvironment) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase("deployment.environment", "", input)
	returnKey = "deployment_environment"

	if returnVal == "" {
		returnVal = agent.Global_Config.DeploymentEnvironment
	}

	// Set global metric deployment environment
	GlobalMetricConfig.DeploymentEnvironment = returnVal.(string)
	return
}

func init() {
	RegisterRule("deployment.environment", new(DeploymentEnvironment))
}
