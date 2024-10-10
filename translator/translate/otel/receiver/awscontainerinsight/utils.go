// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscontainerinsight

import (
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	BaseContainerInsights = iota + 1
)

func EnhancedContainerInsightsEnabled(conf *confmap.Conf) bool {
	isSet := common.GetOrDefaultBool(conf, common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnhancedContainerInsights), false)
	if !isSet {
		levelFloat := common.GetOrDefaultNumber(conf, common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.ContainerInsightsMetricGranularity), 1)
		if levelFloat > BaseContainerInsights {
			isSet = true
		}
	}
	return isSet
}

func AcceleratedComputeMetricsEnabled(conf *confmap.Conf) bool {
	return common.GetOrDefaultBool(conf, common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnableAcceleratedComputeMetric), true)
}
