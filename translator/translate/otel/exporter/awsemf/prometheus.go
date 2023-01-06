// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"go.opentelemetry.io/collector/confmap"
)

func setPrometheusNamespace(conf map[string]interface{}, cfg *awsemfexporter.Config) {
	if namespace, ok := conf["metric_namespace"]; ok {
		cfg.Namespace = namespace.(string)
	}
}

func setPrometheusMetricDescriptors(conf map[string]interface{}, cfg *awsemfexporter.Config) error {
	if mus, ok := conf["metric_unit"]; ok {
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
			return fmt.Errorf("unable to unmarshal metric_declarations: %w", err)
		}
	}
	return nil
}

func setPrometheusMetricDeclarations(conf map[string]interface{}, cfg *awsemfexporter.Config) error {
	if mds, ok := conf["metric_declaration"]; ok {
		metricDeclarations := mds.([]interface{})
		var declarations []map[string]interface{}
		for _, md := range metricDeclarations {
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
			sourceLabels, ok1 := metricDeclaration["source_labels"]
			labelMatcher, ok2 := metricDeclaration["label_matcher"]
			if ok1 && ok2 {
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
	}
	return nil
}
