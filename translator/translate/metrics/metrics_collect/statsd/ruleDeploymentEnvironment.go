// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statsd

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics"
)

type DeploymentEnvironment struct {
}

const SectionkeyDeploymentEnvironment = "deployment.environment"

func (obj *DeploymentEnvironment) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase(SectionkeyDeploymentEnvironment, "", input)
	returnKey = "deployment_environment"

	if returnVal == "" {
		returnVal = metrics.GlobalMetricConfig.DeploymentEnvironment
	}
	return
}

func init() {
	obj := new(DeploymentEnvironment)
	RegisterRule(SectionkeyDeploymentEnvironment, obj)
}
