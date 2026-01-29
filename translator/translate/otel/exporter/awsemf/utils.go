// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"go.opentelemetry.io/collector/confmap"
)

const (
	metricUnit      = "metric_unit"
	metricNamespace = "metric_namespace"
)

// setMetricDescriptors is a shared function to set metric descriptors from metric_unit configuration
func setMetricDescriptors(conf *confmap.Conf, metricUnitKey string, cfg *awsemfexporter.Config) error {
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

// setNamespaceWithDefault is a shared function to set namespace from config or use a default
func setNamespaceWithDefault(conf *confmap.Conf, namespaceKey string, defaultNamespace string, cfg *awsemfexporter.Config) error {
	if namespace, ok := common.GetString(conf, namespaceKey); ok {
		cfg.Namespace = namespace
		return nil
	}

	if defaultNamespace != "" {
		cfg.Namespace = defaultNamespace
	}
	// If defaultNamespace is empty, the namespace from awsemf_default_generic.yaml will be used

	return nil
}
