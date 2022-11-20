// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"fmt"
	"log"
)

type MetricDecorationConfig struct {
	Category string `toml:"category"`
	Metric   string `toml:"name"`
	Rename   string `toml:"rename"`
	Unit     string `toml:"unit"`
}

func NewMetricDecorations(metricConfigs []MetricDecorationConfig) (*MetricDecorations, error) {
	result := &MetricDecorations{
		decorationNames: make(map[string]map[string]string),
		decorationUnits: make(map[string]map[string]string),
	}

	for category, categoryMetrics := range metricDefaultUnit {
		for metricIndex := 0; metricIndex < len(categoryMetrics.supportedMetrics); metricIndex++ {
			supportedMetric := categoryMetrics.supportedMetrics[metricIndex]
			supportedMetricUnit := categoryMetrics.defaultMetricsUnit[metricIndex]
			err := result.addDecorations(category, supportedMetric, "", supportedMetricUnit)
			if err != nil {
				return result, err
			}
		}
	}

	for _, metricConfig := range metricConfigs {
		err := result.addDecorations(metricConfig.Category, metricConfig.Metric, metricConfig.Rename, metricConfig.Unit)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

type MetricDecorations struct {
	decorationNames map[string]map[string]string
	decorationUnits map[string]map[string]string
}

func (m *MetricDecorations) getUnit(category string, metric string) string {
	if val, ok := m.decorationUnits[category]; ok {
		return val[metric]
	}
	return ""
}

func (m *MetricDecorations) getRename(category string, metric string) string {
	if val, ok := m.decorationNames[category]; ok {
		return val[metric]
	}
	return ""
}

func (m *MetricDecorations) addDecorations(category string, name string, rename string, unit string) error {
	if category == "" || name == "" {
		log.Println("W! Metric config miss key identification... ")
		return nil
	}

	if rename != "" {
		val, ok := m.decorationNames[category]
		if !ok {
			val = make(map[string]string)
			m.decorationNames[category] = val
		}
		val[name] = rename
	}

	if unit != "" {
		if isUnitInvalid(unit) {
			return fmt.Errorf("detect unsupported unit")
		}

		val, ok := m.decorationUnits[category]
		if !ok {
			val = make(map[string]string)
			m.decorationUnits[category] = val
		}
		val[name] = unit
	}
	return nil
}
