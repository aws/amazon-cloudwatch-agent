// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscontainerinsight

import (
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	BaseContainerInsights = iota + 1
	EnhancedClusterMetrics
	IndividualPodContainerMetrics
)

type GranularityLevel int

func GetGranularityLevel(conf *confmap.Conf) GranularityLevel {
	levelFloat := common.GetOrDefaultNumber(conf, common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.ContainerInsightsMetricGranularity), 1)

	return GranularityLevel(int(levelFloat))
}
