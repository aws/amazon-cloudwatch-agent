// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscontainerinsight

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"go.opentelemetry.io/collector/confmap"
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
