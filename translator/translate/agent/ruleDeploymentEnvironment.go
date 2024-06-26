// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type DeploymentEnvironment struct{}

func (f *DeploymentEnvironment) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, result := translator.DefaultCase("deployment.environment", "", input)

	// Set global agent deployment environment
	Global_Config.DeploymentEnvironment = result.(string)
	return
}

func init() {
	f := new(DeploymentEnvironment)
	RegisterRule("deployment.environment", f)
}
