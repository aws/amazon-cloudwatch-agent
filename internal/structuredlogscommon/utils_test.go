// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package structuredlogscommon

import (
	"testing"
	"time"

	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"
)

func TestCleanupRules_ValidRules(t *testing.T) {
	m := metric.New("prometheus",
		map[string]string{"tagA": "a", "tagB": "b"},
		map[string]interface{}{"fieldA": 0.1, "fieldB": 0.2},
		time.Now())

	rules := []MetricRule{MetricRule{
		Namespace:     "ns",
		DimensionSets: [][]string{{"tagA", "tagB"}, {"tagA"}, {"tagA", "tagC"}},
		Metrics: []MetricAttr{
			{Unit: "Bytes", Name: "fieldA"},
			{Unit: "Bytes", Name: "fieldC"}},
	},
		MetricRule{
			Namespace:     "ns",
			DimensionSets: [][]string{{"tagA", "tagB"}, {"tagA"}, {"tagC", "tagD"}},
			Metrics: []MetricAttr{
				{Unit: "Bytes", Name: "fieldA"},
				{Unit: "Bytes", Name: "fieldB"}},
		}}

	filteredRule := cleanupRules(m, rules)

	assert.Equal(t, 2, len(filteredRule))
	assert.Equal(t, "ns", filteredRule[0].Namespace)
	assert.Equal(t, []MetricAttr{{Unit: "Bytes", Name: "fieldA"}}, filteredRule[0].Metrics)
	assert.Equal(t, [][]string{{"tagA", "tagB"}, {"tagA"}}, filteredRule[0].DimensionSets)
	assert.Equal(t, []MetricAttr{{Unit: "Bytes", Name: "fieldA"}, {Unit: "Bytes", Name: "fieldB"}}, filteredRule[1].Metrics)
	assert.Equal(t, [][]string{{"tagA", "tagB"}, {"tagA"}}, filteredRule[1].DimensionSets)
}

func TestCleanupRules_EmptyRules_EmptyMetrics(t *testing.T) {
	m := metric.New("prometheus",
		map[string]string{"tagA": "a", "tagB": "b"},
		map[string]interface{}{"fieldA": 0.1, "fieldB": 0.2},
		time.Now())

	rules := []MetricRule{MetricRule{
		Namespace:     "ns",
		DimensionSets: [][]string{{"tagA", "tagB"}, {"tagA"}, {"tagC", "tagD"}},
		Metrics: []MetricAttr{
			{Unit: "Bytes", Name: "fieldC"},
			{Unit: "Bytes", Name: "fieldD"}},
	}}
	filteredRule := cleanupRules(m, rules)

	assert.Equal(t, 0, len(filteredRule))
}

func TestCleanupRules_EmptyRules_EmptyDimension(t *testing.T) {
	m := metric.New("prometheus",
		map[string]string{"tagA": "a", "tagB": "b"},
		map[string]interface{}{"fieldA": 0.1, "fieldB": 0.2},
		time.Now())

	rules := []MetricRule{MetricRule{
		Namespace: "ns1",
		Metrics: []MetricAttr{
			{Unit: "Bytes", Name: "fieldA"},
			{Unit: "Bytes", Name: "fieldB"}},
		DimensionSets: [][]string{{"tagA", "tagC"}, {"tagD"}, {"tagE", "tagF"}},
	}}
	filteredRule := cleanupRules(m, rules)
	assert.Equal(t, 0, len(filteredRule))
}

func TestGetDuplicateMetrics_NoDuplicates(t *testing.T) {
	rules := []MetricRule{MetricRule{
		Namespace: "ns",
		Metrics: []MetricAttr{
			{Unit: "Bytes", Name: "fieldA"},
			{Unit: "Bytes", Name: "fieldB"}},
		DimensionSets: [][]string{{"tagA", "tagB"}, {"tagA", "tagC"}},
	},
		MetricRule{
			Namespace: "ns",
			Metrics: []MetricAttr{
				{Unit: "Bytes", Name: "fieldC"},
				{Unit: "Bytes", Name: "fieldD"}},
			DimensionSets: [][]string{{"tagA", "tagC"}, {"tagC", "tagD"}},
		},
		MetricRule{
			Namespace: "ns",
			Metrics: []MetricAttr{
				{Unit: "Bytes", Name: "fieldE"}},
			DimensionSets: [][]string{{"tagB", "tagD"}},
		}}

	dm := getOverlapMetrics(rules)
	assert.Equal(t, 0, len(dm))
}

func TestGetDuplicateMetrics_Duplicates(t *testing.T) {
	rules := []MetricRule{MetricRule{
		Namespace: "ns",
		Metrics: []MetricAttr{
			{Unit: "Bytes", Name: "fieldA"},
			{Unit: "Bytes", Name: "fieldB"}},
		DimensionSets: [][]string{{"tagA", "tagB"}, {"tagA"}, {"tagA", "tagC"}},
	},
		MetricRule{
			Namespace: "ns",
			Metrics: []MetricAttr{
				{Unit: "Bytes", Name: "fieldA"},
				{Unit: "Bytes", Name: "fieldC"},
				{Unit: "Bytes", Name: "fieldD"}},
			DimensionSets: [][]string{{"tagA", "tagC"}, {"tagA"}, {"tagC", "tagD"}},
		},
		MetricRule{
			Namespace: "ns",
			Metrics: []MetricAttr{
				{Unit: "Bytes", Name: "fieldA"},
				{Unit: "Bytes", Name: "fieldC"},
				{Unit: "Bytes", Name: "fieldE"}},
			DimensionSets: [][]string{{"tagA", "tagC"}, {"tagA"}, {"tagB", "tagD"}},
		}}

	dm := getOverlapMetrics(rules)
	assert.Equal(t, 2, len(dm))

	rulesa, oka := dm[MetricAttr{Unit: "Bytes", Name: "fieldA"}]
	assert.True(t, oka)
	assert.Equal(t, 3, len(rulesa))

	rulesb, okb := dm[MetricAttr{Unit: "Bytes", Name: "fieldC"}]
	assert.True(t, okb)
	assert.Equal(t, 2, len(rulesb))
}

func TestMergeOverlapMetrics_Duplicates(t *testing.T) {
	duplicateMetrics := make(map[MetricAttr][]*MetricRule)

	rules := []*MetricRule{&MetricRule{
		Namespace: "ns",
		Metrics: []MetricAttr{
			{Unit: "Bytes", Name: "fieldA"},
			{Unit: "Bytes", Name: "fieldB"}},
		DimensionSets: [][]string{{"tagA", "tagB"}, {"tagA"}, {"tagA", "tagC"}},
	},
		&MetricRule{
			Namespace: "ns",
			Metrics: []MetricAttr{
				{Unit: "Bytes", Name: "fieldA"},
				{Unit: "Bytes", Name: "fieldC"},
				{Unit: "Bytes", Name: "fieldD"}},
			DimensionSets: [][]string{{"tagA", "tagC"}, {"tagA"}, {"tagC", "tagD"}},
		},
		&MetricRule{
			Namespace: "ns",
			Metrics: []MetricAttr{
				{Unit: "Bytes", Name: "fieldA"},
				{Unit: "Bytes", Name: "fieldC"},
				{Unit: "Bytes", Name: "fieldE"}},
			DimensionSets: [][]string{{"tagA", "tagC"}, {"tagA"}, {"tagB", "tagD"}},
		}}

	duplicateMetrics[MetricAttr{Unit: "Bytes", Name: "fieldA"}] = rules
	duplicateMetrics[MetricAttr{Unit: "Bytes", Name: "fieldC"}] = rules[1:]

	mrs := mergeDuplicateMetrics(duplicateMetrics)

	assert.Equal(t, 2, len(mrs))
	mra := mrs[MetricAttr{Unit: "Bytes", Name: "fieldA"}]
	assert.Equal(t, "ns", mra.Namespace)
	assert.Equal(t, []MetricAttr{MetricAttr{Unit: "Bytes", Name: "fieldA"}}, mra.Metrics)
	assert.Equal(t, [][]string{{"tagA", "tagB"}, {"tagA"}, {"tagA", "tagC"}, {"tagC", "tagD"}, {"tagB", "tagD"}}, mra.DimensionSets)

	mrc := mrs[MetricAttr{Unit: "Bytes", Name: "fieldC"}]
	assert.Equal(t, "ns", mrc.Namespace)
	assert.Equal(t, []MetricAttr{MetricAttr{Unit: "Bytes", Name: "fieldC"}}, mrc.Metrics)
	assert.Equal(t, [][]string{{"tagA", "tagC"}, {"tagA"}, {"tagC", "tagD"}, {"tagB", "tagD"}}, mrc.DimensionSets)
}

func TestMergeOverlapMetrics_NoDuplicates(t *testing.T) {
	duplicateMetrics := make(map[MetricAttr][]*MetricRule)

	rules := []*MetricRule{&MetricRule{
		Namespace: "ns",
		Metrics: []MetricAttr{
			{Unit: "Bytes", Name: "fieldA"},
			{Unit: "Bytes", Name: "fieldB"}},
		DimensionSets: [][]string{{"tagA", "tagB"}, {"tagA"}, {"tagA", "tagC"}},
	},
		&MetricRule{
			Namespace: "ns",
			Metrics: []MetricAttr{
				{Unit: "Bytes", Name: "fieldA"},
				{Unit: "Bytes", Name: "fieldC"},
				{Unit: "Bytes", Name: "fieldD"}},
			DimensionSets: [][]string{{"tagC", "tagD"}},
		},
		&MetricRule{
			Namespace: "ns",
			Metrics: []MetricAttr{
				{Unit: "Bytes", Name: "fieldA"},
				{Unit: "Bytes", Name: "fieldC"},
				{Unit: "Bytes", Name: "fieldE"}},
			DimensionSets: [][]string{{"tagB", "tagC"}, {"tagB", "tagD"}},
		}}

	duplicateMetrics[MetricAttr{Unit: "Bytes", Name: "fieldA"}] = rules
	merged := mergeDuplicateMetrics(duplicateMetrics)
	assert.Equal(t, 0, len(merged))
}

func Test_getDeduppedRules(t *testing.T) {
	rules := []MetricRule{MetricRule{
		Namespace: "ns",
		Metrics: []MetricAttr{
			{Unit: "Bytes", Name: "fieldA"},
			{Unit: "Bytes", Name: "fieldB"}},
		DimensionSets: [][]string{{"tagA", "tagB"}, {"tagA"}, {"tagA", "tagC"}},
	}}

	mergedRules := make(map[MetricAttr]*MetricRule)
	mergedRules[MetricAttr{Unit: "Bytes", Name: "fieldA"}] = &MetricRule{
		Namespace: "ns",
		Metrics: []MetricAttr{
			{Unit: "Bytes", Name: "fieldA"}},
		DimensionSets: [][]string{{"tagA", "tagB"}, {"tagA", "tagC"}},
	}

	res := getDeduppedRules(rules, mergedRules)
	assert.Equal(t, 2, len(res))

	assert.Equal(t, []MetricAttr{MetricAttr{Unit: "Bytes", Name: "fieldA"}}, res[0].Metrics)
	assert.Equal(t, [][]string{{"tagA", "tagB"}, {"tagA", "tagC"}}, res[0].DimensionSets)
	assert.Equal(t, []MetricAttr{MetricAttr{Unit: "Bytes", Name: "fieldB"}}, res[1].Metrics)
	assert.Equal(t, [][]string{{"tagA", "tagB"}, {"tagA"}, {"tagA", "tagC"}}, res[1].DimensionSets)
}
