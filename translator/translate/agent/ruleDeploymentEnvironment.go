// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type DeploymentEnvironment struct {
}

func (f *DeploymentEnvironment) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase("deployment.environment", "", input)
	returnKey = "deployment_environment"
	// Set global agent deployment environment
	Global_Config.DeploymentEnvironment = returnVal.(string)
	return
}
