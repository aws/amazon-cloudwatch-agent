// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"log"
)

type MetricDecorationConfig struct {
	Category string `mapstructure:"category,omitempty"`
	Metric   string `mapstructure:"name,omitempty"`
	Rename   string `mapstructure:"rename,omitempty"`
	Unit     string `mapstructure:"unit,omitempty"`
}

func NewMetricDecorations(metricConfigs []MetricDecorationConfig) (*MetricDecorations, error) {
	result := &MetricDecorations{
		decorationNames: make(map[string]map[string]string),
		decorationUnits: make(map[string]map[string]string),
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
		if !isCloudWatchStandardUnit(unit) {
			return fmt.Errorf("detected unsupported unit: %s", unit)
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

var standardUnits = map[string]struct{}{}

func isCloudWatchStandardUnit(unit string) bool {
	if unit == "" {
		return false
	}
	_, ok := standardUnits[unit]
	return ok
}

func init() {
	for _, unit := range types.StandardUnitNone.Values() {
		standardUnits[string(unit)] = struct{}{}
	}
}
