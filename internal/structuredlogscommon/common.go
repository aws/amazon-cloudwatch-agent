// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package structuredlogscommon

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"

	"github.com/influxdata/telegraf"
)

// Used by StructuredLog adapted-processor plugins.

const (
	attributesInFields = "attributesInFields"
	MetricRuleKey      = "CloudWatchMetrics"
)

// Some attributes are not in type string, use fields to pass these tags
func AppendAttributesInFields(fieldName string, fieldValue interface{}, metric telegraf.Metric) {
	metric.AddField(fieldName, fieldValue)
	if val, ok := metric.Tags()[attributesInFields]; ok {
		val += fmt.Sprintf(",%s", fieldName)
		metric.AddTag(attributesInFields, val)
	} else {
		metric.AddTag(attributesInFields, fieldName)
	}
}

func BuildAttributes(metric telegraf.Metric, structuredLogContent map[string]interface{}) {
	mTags := metric.Tags()
	// build all the attributesInFields
	if val, ok := mTags[attributesInFields]; ok {
		attributes := strings.Split(val, ",")
		mFields := metric.Fields()
		for _, attr := range attributes {
			if fieldVal, ok := mFields[attr]; ok {
				structuredLogContent[attr] = fieldVal
				metric.RemoveField(attr)
			}
		}
		metric.RemoveTag(attributesInFields)
		delete(mTags, attributesInFields)
	}

	// build remaining attributes
	for k := range mTags {
		structuredLogContent[k] = mTags[k]
	}
}

func BuildMeasurements(metric telegraf.Metric, structuredLogContent map[string]interface{}) error {
	for k, v := range metric.Fields() {
		var value interface{}

		switch t := v.(type) {
		case int:
			value = float64(t)
		case int32:
			value = float64(t)
		case int64:
			value = float64(t)
		case float64:
			value = t
		case bool:
			value = t
		case string:
			value = t
		case time.Time:
			value = float64(t.Unix())

		default:
			return fmt.Errorf("detect unexpected fields (%s,%v) when encoding structured log event", k, v)
		}
		structuredLogContent[k] = value
	}
	return nil
}

// Add structured log schema version
func AddVersion(metric telegraf.Metric) {
	metric.AddTag("Version", "0")
}

type MetricRule struct {
	Metrics       []MetricAttr `json:"Metrics"`
	DimensionSets [][]string   `json:"Dimensions"`
	Namespace     string       `json:"Namespace"`
}

type MetricAttr struct {
	Unit string `json:"Unit,omitempty"`
	Name string `json:"Name"`
}

func AttachMetricRule(metric telegraf.Metric, staticMetricRule map[string][]MetricRule, cloudwatchNamespace string) {
	rules, ok := staticMetricRule[metric.Tags()[containerinsightscommon.MetricType]]
	if !ok {
		return
	}

	var filterredRule []MetricRule
	for _, rule := range rules {
		var filteredMetrics []MetricAttr
		var filteredDimensionSets [][]string
		for _, ruleMetric := range rule.Metrics {
			if metric.HasField(ruleMetric.Name) {
				filteredMetrics = append(filteredMetrics, ruleMetric)
			}
		}

		for _, ruleDimensionSet := range rule.DimensionSets {
			anyDimensionMiss := false
			for _, ruleDimension := range ruleDimensionSet {
				if !metric.HasTag(ruleDimension) {
					anyDimensionMiss = true
				}
			}
			if !anyDimensionMiss {
				filteredDimensionSets = append(filteredDimensionSets, ruleDimensionSet)
			}
		}
		// If dimension doesn't exactly match or no metric remain after filter, skip it
		if len(filteredMetrics) > 0 && len(filteredDimensionSets) > 0 {
			filterredRule = append(filterredRule, MetricRule{Metrics: filteredMetrics, DimensionSets: filteredDimensionSets, Namespace: cloudwatchNamespace})
		}
	}

	if len(filterredRule) > 0 {
		AppendAttributesInFields(MetricRuleKey, filterredRule, metric)
	}
}
