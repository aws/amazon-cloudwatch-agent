// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
)

type DeploymentEnvironment struct {
}

func (f *DeploymentEnvironment) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase("deployment.environment", "", input)
	returnKey = "deployment_environment"

	if returnVal == "" {
		returnVal = logs.GlobalLogConfig.DeploymentEnvironment
	}

	return
}

func init() {
	f := new(DeploymentEnvironment)
	r := []Rule{f}
	RegisterRule("deployment.environment", r)
}
