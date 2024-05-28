// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"strings"

	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/metric"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/config"
)

const (
	dropOriginalWildcard = "*"
)

func GetRollupDimensions(conf *confmap.Conf) [][]string {
	key := ConfigKey(MetricsKey, AggregationDimensionsKey)
	value := conf.Get(key)
	if value == nil {
		return nil
	}
	aggregates, ok := value.([]interface{})
	if !ok || !isValidRollupList(aggregates) {
		return nil
	}
	rollup := make([][]string, len(aggregates))
	for i, aggregate := range aggregates {
		dimensions := aggregate.([]interface{})
		rollup[i] = make([]string, len(dimensions))
		for j, dimension := range dimensions {
			rollup[i][j] = dimension.(string)
		}
	}
	return rollup
}

// isValidRollupList confirms whether the supplied aggregate_dimension is a valid type ([][]string)
func isValidRollupList(aggregates []interface{}) bool {
	if len(aggregates) == 0 {
		return false
	}
	for _, aggregate := range aggregates {
		if dimensions, ok := aggregate.([]interface{}); ok {
			if len(dimensions) != 0 {
				for _, dimension := range dimensions {
					if _, ok := dimension.(string); !ok {
						return false
					}
				}
			}
		} else {
			return false
		}
	}

	return true
}

func GetDropOriginalMetrics(conf *confmap.Conf) map[string]bool {
	key := ConfigKey(MetricsKey, MetricsCollectedKey)
	value := conf.Get(key)
	if value == nil {
		return nil
	}
	categories := value.(map[string]interface{})
	dropOriginalMetrics := make(map[string]bool)
	for category := range categories {
		realCategoryName := config.GetRealPluginName(category)
		measurementCfgKey := ConfigKey(key, category, MeasurementKey)
		dropOriginalCfgKey := ConfigKey(key, category, DropOriginalMetricsKey)
		/* Drop original metrics does not support procstat since procstat can monitor multiple process
		   		"procstat": [
		           {
		             "exe": "W3SVC",
		             "measurement": [
		               "pid_count"
		             ]
		           },
		           {
		             "exe": "IISADMIN",
		             "measurement": [
		               "pid_count"
		             ]
		           }]
		   	Therefore, dropping the original metrics can conflict between these two processes (e.g customers can drop pid_count with the first
		   	process but not the second process)
		*/
		if dropMetrics := GetArray[any](conf, dropOriginalCfgKey); dropMetrics != nil {
			for _, dropMetric := range dropMetrics {
				measurements := GetArray[any](conf, measurementCfgKey)
				if measurements == nil {
					continue
				}

				dropMetricStr, ok := dropMetric.(string)
				if !ok {
					continue
				}

				if !strings.Contains(dropMetricStr, category) && dropMetricStr != dropOriginalWildcard {
					dropMetricStr = metric.DecorateMetricName(realCategoryName, dropMetricStr)
				}
				isMetricDecoration := false
				for _, measurement := range measurements {
					switch val := measurement.(type) {
					/*
						 "disk": {
							"measurement": [
								{
									"name": "free",
									"rename": "DISK_FREE",
									"unit": "unit"
								}
							]
						}
					*/
					case map[string]interface{}:
						metricName, ok := val["name"].(string)
						if !ok {
							continue
						}
						if !strings.Contains(metricName, category) {
							metricName = metric.DecorateMetricName(realCategoryName, metricName)
						}
						// If customers provides drop_original_metrics with a wildcard (*), adding the renamed metric or add the original metric
						// if customers only re-unit the metric
						if strings.Contains(dropMetricStr, metricName) || dropMetricStr == dropOriginalWildcard {
							isMetricDecoration = true
							if newMetricName, ok := val["rename"].(string); ok {
								dropOriginalMetrics[newMetricName] = true
							} else {
								dropOriginalMetrics[metricName] = true
							}
						}

					/*
						"measurement": ["free"]
					*/
					case string:
						if dropMetricStr != dropOriginalWildcard {
							continue
						}
						metricName := val
						if !strings.Contains(metricName, category) {
							metricName = metric.DecorateMetricName(realCategoryName, metricName)
						}

						dropOriginalMetrics[metricName] = true
					default:
						continue
					}
				}

				if !isMetricDecoration && dropMetricStr != dropOriginalWildcard {
					dropOriginalMetrics[dropMetricStr] = true
				}

			}
		}
	}
	return dropOriginalMetrics
}
