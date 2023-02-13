// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emfprocessor

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/context"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util/ecsutil"
)

const (
	SectionKeyMetricNamespace = "metric_namespace"

	ECSDefaultCloudWatchNamespace = "ECS/ContainerInsights/Prometheus"
	K8SDefaultCloudWatchNamespace = "ContainerInsights/Prometheus"
	EC2DefaultCloudWatchNamespace = "CWAgent/Prometheus"
)

type MetricNamespace struct {
}

func (d *MetricNamespace) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKeyMetricNamespace, "", input)
	if returnVal != "" {
		return
	}

	if context.CurrentContext().RunInContainer() {
		if ecsutil.GetECSUtilSingleton().IsECS() {
			returnVal = ECSDefaultCloudWatchNamespace
		} else {
			returnVal = K8SDefaultCloudWatchNamespace
		}
	} else {
		returnVal = EC2DefaultCloudWatchNamespace
	}
	return
}

func init() {
	RegisterRule(SectionKeyMetricNamespace, new(MetricNamespace))
}
