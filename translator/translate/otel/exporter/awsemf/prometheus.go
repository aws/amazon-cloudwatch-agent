// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const (
	metricUnit                    = "metric_unit"
	metricNamespace               = "metric_namespace"
	metricDeclartion              = "metric_declaration"
	ecsDefaultCloudWatchNamespace = "ECS/ContainerInsights/Prometheus"
	k8sDefaultCloudWatchNamespace = "ContainerInsights/Prometheus"
	ec2DefaultCloudWatchNamespace = "CWAgent/Prometheus"
	eksDefaultLogGroupFormat      = "/aws/containerinsights/%s/prometheus"
	ecsDefaultLogGroupFormat      = "/aws/ecs/containerinsights/%s/prometheus"
)

func setPrometheusLogGroup(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	if logGroupName, ok := common.GetString(conf, common.ConfigKey(prometheusBasePathKey, common.LogGroupName)); ok {
		cfg.LogGroupName = logGroupName
		return nil
	}

	if context.CurrentContext().RunInContainer() {
		if ecsutil.GetECSUtilSingleton().IsECS() {
			if clusterName := ecsutil.GetECSUtilSingleton().Cluster; clusterName != "" {
				cfg.LogGroupName = fmt.Sprintf(ecsDefaultLogGroupFormat, clusterName)
			}
		} else {

			if clusterName := util.GetClusterNameFromEc2Tagger(); clusterName != "" {
				cfg.LogGroupName = fmt.Sprintf(eksDefaultLogGroupFormat, clusterName)
			}
		}
	}

	if cfg.LogGroupName == "" {
		return errors.New("prometheus does not have log group name. For more information, please follow this document https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-PrometheusEC2.html#CloudWatch-Agent-PrometheusEC2-configure")
	}
	return nil
}
func setPrometheusNamespace(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	if namespace, ok := common.GetString(conf, common.ConfigKey(emfProcessorBasePathKey, metricNamespace)); ok {
		cfg.Namespace = namespace
		return nil
	}

	if context.CurrentContext().RunInContainer() {
		if ecsutil.GetECSUtilSingleton().IsECS() {
			cfg.Namespace = ecsDefaultCloudWatchNamespace
		} else {
			cfg.Namespace = k8sDefaultCloudWatchNamespace
		}
	} else {
		cfg.Namespace = ec2DefaultCloudWatchNamespace
	}

	return nil

}

func setPrometheusMetricDescriptors(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	metricUnitKey := common.ConfigKey(emfProcessorBasePathKey, metricUnit)
	if !conf.IsSet(metricUnitKey) {
		return nil
	}

	mus := conf.Get(metricUnitKey)
	metricUnits := mus.(map[string]interface{})
	var metricDescriptors []map[string]string
	for mName, unit := range metricUnits {
		metricDescriptors = append(metricDescriptors, map[string]string{
			"metric_name": mName,
			"unit":        unit.(string),
		})
	}
	c := confmap.NewFromStringMap(map[string]interface{}{
		"metric_descriptors": metricDescriptors,
	})
	cfg.MetricDescriptors = []awsemfexporter.MetricDescriptor{}
	if err := c.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("unable to unmarshal metric_descriptors: %w", err)
	}
	return nil
}

func setPrometheusMetricDeclarations(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	metricDeclarationKey := common.ConfigKey(emfProcessorBasePathKey, metricDeclartion)
	if !conf.IsSet(metricDeclarationKey) {
		return nil
	}
	metricDeclarations := conf.Get(metricDeclarationKey)
	var declarations []map[string]interface{}
	for _, md := range metricDeclarations.([]interface{}) {
		metricDeclaration := md.(map[string]interface{})
		declaration := map[string]interface{}{}
		if dimensions, ok := metricDeclaration["dimensions"]; ok {
			declaration["dimensions"] = dimensions
		}
		if metricSelectors, ok := metricDeclaration["metric_selectors"]; ok {
			declaration["metric_name_selectors"] = metricSelectors
		} else {
			// If no metric selectors are provided, that particular metric declaration is invalid
			continue
		}
		labelMatcher, ok := metricDeclaration["label_matcher"]
		if !ok {
			labelMatcher = ".*"
		}
		sourceLabels, ok := metricDeclaration["source_labels"]
		if ok {
			// OTel awsemfexporter allows specifying multiple label_matchers but CWA only allows specifying one
			declaration["label_matchers"] = [...]map[string]interface{}{
				{
					"label_names": sourceLabels,
					"regex":       labelMatcher,
				},
			}
		} else {
			// If no source labels or label matchers are provided, that particular metric declaration is invalid
			continue
		}
		declarations = append(declarations, declaration)
	}
	c := confmap.NewFromStringMap(map[string]interface{}{
		"metric_declarations": declarations,
	})
	cfg.MetricDeclarations = []*awsemfexporter.MetricDeclaration{} // Clear out any existing declarations
	if err := c.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("unable to unmarshal metric_declarations: %w", err)
	}
	return nil
}
