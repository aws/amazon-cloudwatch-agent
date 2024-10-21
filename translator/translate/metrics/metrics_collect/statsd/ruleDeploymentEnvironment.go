// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statsd

import (
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity/entityattributes"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type DeploymentEnvironment struct {
}

const SectionkeyDeploymentEnvironment = "deployment.environment"

func (obj *DeploymentEnvironment) ApplyRule(input interface{}) (string, interface{}) {
	returnKey, returnVal := translator.DefaultCase(SectionkeyDeploymentEnvironment, "", input)

	parentKeyVal := metrics.GlobalMetricConfig.DeploymentEnvironment
	if returnVal != "" {
		return common.Tags, map[string]interface{}{returnKey: returnVal, entityattributes.AttributeDeploymentEnvironmentSource: entityattributes.AttributeServiceNameSourceUserConfig}
	} else if parentKeyVal != "" {
		return common.Tags, map[string]interface{}{returnKey: parentKeyVal, entityattributes.AttributeDeploymentEnvironmentSource: entityattributes.AttributeServiceNameSourceUserConfig}
	}
	return "", nil
}

func init() {
	obj := new(DeploymentEnvironment)
	RegisterRule(SectionkeyDeploymentEnvironment, obj)
}
