// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func setKubernetesKueueMetricDeclaration(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	cfg.MetricDeclarations = getKueueMetricDeclarations(conf)
	return nil
}

func getKueueMetricDeclarations(conf *confmap.Conf) []*awsemfexporter.MetricDeclaration {
	var metricDeclarations []*awsemfexporter.MetricDeclaration
	if common.KueueContainerInsightsEnabled(conf) {
		metricDeclarations = []*awsemfexporter.MetricDeclaration{
			{
				Dimensions: [][]string{
					{"ClusterName"},
					{"ClusterName", "ClusterQueue"},
					{"ClusterName", "ClusterQueue", "Status"},
					{"ClusterName", "Status"},
				},
				MetricNameSelectors: []string{
					"kueue_pending_workloads",
				},
			},
			{
				Dimensions: [][]string{
					{"ClusterName"},
					{"ClusterName", "ClusterQueue"},
					{"ClusterName", "ClusterQueue", "Reason"},
					{"ClusterName", "Reason"},
				},
				MetricNameSelectors: []string{
					"kueue_evicted_workloads_total",
				},
			},
			{
				Dimensions: [][]string{
					{"ClusterName"},
					{"ClusterName", "ClusterQueue"},
				},
				MetricNameSelectors: []string{
					"kueue_admitted_active_workloads",
				},
			},
			{
				Dimensions: [][]string{
					{"ClusterName"},
					{"ClusterName", "ClusterQueue"},
					{"ClusterName", "ClusterQueue", "Resource"},
					{"ClusterName", "ClusterQueue", "Resource", "Flavor"},
					{"ClusterName", "ClusterQueue", "Flavor"},
				},
				MetricNameSelectors: []string{
					"kueue_cluster_queue_resource_usage",
					"kueue_cluster_queue_nominal_quota",
				},
			},
		}
	}
	return metricDeclarations
}
