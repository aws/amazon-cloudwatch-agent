// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package structuredlogsadapter

import (
	"fmt"

	"github.com/influxdata/telegraf"

	. "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/internal/structuredlogscommon"
)

func TagMetricSource(metric telegraf.Metric) {
	metricType, ok := metric.Tags()[MetricType]
	if !ok {
		return
	}

	var sources []string
	switch metricType {
	case TypeNode:
		sources = append(sources, []string{"cadvisor", "/proc", "pod", "calculated"}...)
	case TypeNodeFS:
		sources = append(sources, []string{"cadvisor", "calculated"}...)
	case TypeNodeNet:
		sources = append(sources, []string{"cadvisor", "calculated"}...)
	case TypeNodeDiskIO:
		sources = append(sources, []string{"cadvisor"}...)
	case TypePod:
		sources = append(sources, []string{"cadvisor", "pod", "calculated"}...)
	case TypePodNet:
		sources = append(sources, []string{"cadvisor", "calculated"}...)
	case TypeContainer:
		sources = append(sources, []string{"cadvisor", "pod", "calculated"}...)
	case TypeContainerFS:
		sources = append(sources, []string{"cadvisor", "calculated"}...)
	case TypeContainerDiskIO:
		sources = append(sources, []string{"cadvisor"}...)
	case TypeCluster, TypeClusterService, TypeClusterNamespace:
		sources = append(sources, []string{"apiserver"}...)
	}

	if len(sources) > 0 {
		structuredlogscommon.AppendAttributesInFields(SourcesKey, sources, metric)
	}
}

func TagLogGroup(metric telegraf.Metric) {
	logGroup := fmt.Sprintf("/aws/containerinsights/%s/performance", metric.Tags()[ClusterNameKey])
	metric.AddTag(logscommon.LogGroupNameTag, logGroup)
}

func AddKubernetesInfo(metric telegraf.Metric, kubernetesBlob map[string]interface{}) {
	tags := metric.Tags()
	needMoveToKubernetes := map[string]string{ContainerNamekey: "container_name", K8sPodNameKey: "pod_name",
		PodIdKey: "pod_id"}
	needCopyToKubernetes := map[string]string{K8sNamespace: "namespace_name", TypeService: "service_name", NodeNameKey: "host"}

	for k, v := range needMoveToKubernetes {
		if metric.HasTag(k) {
			kubernetesBlob[v] = tags[k]
			metric.RemoveTag(k)
		}
	}
	for k, v := range needCopyToKubernetes {
		if metric.HasTag(k) {
			kubernetesBlob[v] = tags[k]
		}
	}

	if len(kubernetesBlob) > 0 {
		structuredlogscommon.AppendAttributesInFields(Kubernetes, kubernetesBlob, metric)
	}
	structuredlogscommon.AddVersion(metric)
}
