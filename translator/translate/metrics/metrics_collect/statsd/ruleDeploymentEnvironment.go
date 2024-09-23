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

func (obj *DeploymentEnvironment) ApplyRule(input interface{}) (string, interface{}) {
	_, returnVal := translator.DefaultCase(SectionkeyDeploymentEnvironment, "", input)
	returnKey := "deployment.environment"

	if returnVal == "" {
		returnVal = metrics.GlobalMetricConfig.DeploymentEnvironment
	}
	return "tags", map[string]interface{}{returnKey: returnVal}
}

func init() {
	obj := new(DeploymentEnvironment)
	RegisterRule(SectionkeyDeploymentEnvironment, obj)
}
