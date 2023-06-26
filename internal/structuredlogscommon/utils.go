// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package structuredlogscommon

import (
	"reflect"

	"github.com/influxdata/telegraf"
)

func cleanupRules(metric telegraf.Metric, rules []MetricRule) (filteredRule []MetricRule) {
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
			filteredRule = append(filteredRule, MetricRule{Metrics: filteredMetrics, DimensionSets: filteredDimensionSets, Namespace: rule.Namespace})
		}
	}

	return
}

func getOverlapMetrics(rules []MetricRule) map[MetricAttr][]*MetricRule {
	metrics := make(map[MetricAttr]map[*MetricRule]interface{})
	for k, rule := range rules {
		for _, ruleMetric := range rule.Metrics {
			if _, ok := metrics[ruleMetric]; !ok {
				metrics[ruleMetric] = make(map[*MetricRule]interface{})
			}
			metrics[ruleMetric][&rules[k]] = nil
		}
	}

	duplicateMetrics := make(map[MetricAttr][]*MetricRule)
	for k, v := range metrics {
		if len(v) > 1 {
			for rule := range v {
				duplicateMetrics[k] = append(duplicateMetrics[k], rule)
			}
		}
	}
	return duplicateMetrics
}

func dimensionSetInSets(dim []string, dimensionSet [][]string) bool {
	for _, d := range dimensionSet {
		if reflect.DeepEqual(dim, d) {
			return true
		}
	}
	return false
}

func mergeDuplicateMetrics(duplicateMetrics map[MetricAttr][]*MetricRule) map[MetricAttr]*MetricRule {
	mergedRules := make(map[MetricAttr]*MetricRule)
	var ns string
	for metric, rules := range duplicateMetrics {
		merged := false
		var newDimensionSets [][]string
		for _, rule := range rules {
			if ns == "" {
				ns = rule.Namespace
			}

			for _, rd := range rule.DimensionSets {
				if dimensionSetInSets(rd, newDimensionSets) {
					merged = true
				} else {
					newDimensionSets = append(newDimensionSets, rd)
				}
			}
		}

		if merged {
			rule := &MetricRule{
				Namespace:     ns,
				Metrics:       []MetricAttr{metric},
				DimensionSets: newDimensionSets,
			}
			mergedRules[metric] = rule
		}
	}

	return mergedRules
}

func getDeduppedRules(rules []MetricRule, mergedRules map[MetricAttr]*MetricRule) (dedupRules []MetricRule) {
	for _, v := range mergedRules {
		dedupRules = append(dedupRules, *v)
	}

	for _, rule := range rules {
		updatedMetrics := rule.Metrics
		for k, _ := range mergedRules {
			for i, ruleMetric := range updatedMetrics {
				if k == ruleMetric {
					updatedMetrics[i] = updatedMetrics[len(updatedMetrics)-1]
					updatedMetrics = updatedMetrics[:len(updatedMetrics)-1]
					break
				}
			}
		}
		if len(updatedMetrics) > 0 {
			dedupRules = append(dedupRules, MetricRule{Metrics: updatedMetrics, DimensionSets: rule.DimensionSets, Namespace: rule.Namespace})
		}
	}

	return
}

func dedupRules(rules []MetricRule) (dedupRules []MetricRule) {
	duplicateMetrics := getOverlapMetrics(rules)
	if len(duplicateMetrics) == 0 {
		dedupRules = rules
		return
	}

	mergedRules := mergeDuplicateMetrics(duplicateMetrics)
	if len(mergedRules) == 0 {
		dedupRules = rules
		return
	}

	return getDeduppedRules(rules, mergedRules)
}
