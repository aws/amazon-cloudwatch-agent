// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package structuredlogscommon

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/influxdata/telegraf/metric"
	"github.com/stretchr/testify/assert"
)

func TestAppendAttributesInFields(t *testing.T) {
	m := metric.New("test", map[string]string{}, map[string]interface{}{}, time.Now())
	AppendAttributesInFields("testFieldName", "testFieldValue", m)
	assert.Equal(t, "testFieldName", m.Tags()[attributesInFields])
	assert.Equal(t, "testFieldValue", m.Fields()["testFieldName"].(string))

	AppendAttributesInFields("testFieldName2", "testFieldValue2", m)
	assert.Equal(t, "testFieldName,testFieldName2", m.Tags()[attributesInFields])
	assert.Equal(t, "testFieldValue2", m.Fields()["testFieldName2"].(string))
}

func TestBuildAttributes(t *testing.T) {
	m := metric.New("test", map[string]string{}, map[string]interface{}{}, time.Now())
	AppendAttributesInFields("testFieldName", "testFieldValue", m)
	structuredlogs := map[string]interface{}{}
	BuildAttributes(m, structuredlogs)
	assert.Equal(t, map[string]interface{}{"testFieldName": "testFieldValue"}, structuredlogs)
}

func TestBuildValidMeasurements(t *testing.T) {
	m := metric.New("test",
		map[string]string{},
		map[string]interface{}{"testFieldString": "value", "testFieldInt": 0, "testFieldFloat": 0.0, "testFieldBool": true, "testFieldTime": time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)},
		time.Now())
	structuredlogs := map[string]interface{}{}
	err := BuildMeasurements(m, structuredlogs)
	assert.Equal(t, nil, err)
	assert.Equal(t,
		map[string]interface{}{"testFieldString": "value", "testFieldInt": 0.0, "testFieldFloat": 0.0, "testFieldBool": true, "testFieldTime": 1258490098.0},
		structuredlogs)
}

func TestBuildInvalidMeasurements(t *testing.T) {
	m := metric.New("test", map[string]string{}, map[string]interface{}{"testFieldMap": map[string]string{}}, time.Now())
	structuredlogs := map[string]interface{}{}
	err := BuildMeasurements(m, structuredlogs)
	assert.True(t, nil != err)
	assert.Equal(t, map[string]interface{}{}, structuredlogs)
}

func TestAddVersion(t *testing.T) {
	m := metric.New("test", map[string]string{}, map[string]interface{}{"testFieldMap": map[string]string{}}, time.Now())
	AddVersion(m)
	assert.True(t, m.HasTag("Version"))
}

func TestAttachMetricRuleValidRules(t *testing.T) {
	m := metric.New("prometheus",
		map[string]string{"tagA": "a"},
		map[string]interface{}{"fieldA": 0.1},
		time.Now())

	rules := []MetricRule{MetricRule{
		Namespace:     "ns",
		DimensionSets: [][]string{{"tagA", "tagB"}, {"tagA"}},
		Metrics: []MetricAttr{
			{Unit: "Bytes", Name: "fieldA"}},
	}}

	AttachMetricRule(m, rules)
	assert.True(t, m.HasField(MetricRuleKey))
}

func TestAttachMetricRuleInvalidRules(t *testing.T) {
	m := metric.New("prometheus",
		map[string]string{"tagA": "a"},
		map[string]interface{}{"fieldA": 0.1},
		time.Now())

	rules := []MetricRule{MetricRule{
		Namespace:     "ns",
		DimensionSets: [][]string{{"tagC"}},
		Metrics: []MetricAttr{
			{Unit: "Bytes", Name: "fieldA"}},
	}}

	AttachMetricRule(m, rules)
	assert.True(t, !m.HasField(MetricRuleKey))
}

func TestAttachMetricRulewithDedupValidRules(t *testing.T) {
	m := metric.New("prometheus",
		map[string]string{"tagA": "a", "tagB": "b"},
		map[string]interface{}{"fieldA": 0.1, "fieldB": 0.2},
		time.Now())

	var rules []MetricRule
	var rule1 = MetricRule{
		Namespace:     "ns",
		DimensionSets: [][]string{{"tagA", "tagB"}, {"tagA"}},
		Metrics: []MetricAttr{
			{Unit: "Bytes", Name: "fieldA"}},
	}
	rules = append(rules, rule1)
	AttachMetricRuleWithDedup(m, rules)

	assert.True(t, m.HasField(MetricRuleKey))
	fields := m.Fields()[MetricRuleKey]
	filteredRule, _ := fields.([]MetricRule)
	assert.Equal(t, 1, len(filteredRule))
	assert.Equal(t, []MetricAttr{{Unit: "Bytes", Name: "fieldA"}}, filteredRule[0].Metrics)
	assert.Equal(t, [][]string{{"tagA", "tagB"}, {"tagA"}}, filteredRule[0].DimensionSets)
}

func TestAttachMetricRulewithDedupDupRules(t *testing.T) {
	m := metric.New("prometheus",
		map[string]string{"tagA": "a", "tagB": "b", "tagC": "c"},
		map[string]interface{}{"fieldA": 0.1, "fieldB": 0.2, "fieldC": 0.3, "fieldD": 0.3},
		time.Now())

	rules := []MetricRule{
		MetricRule{
			Namespace: "ns",
			Metrics: []MetricAttr{
				{Unit: "Bytes", Name: "fieldA"},
				{Unit: "Bytes", Name: "fieldC"},
				{Unit: "Bytes", Name: "fieldE"}},
			DimensionSets: [][]string{{"tagA", "tagB"}, {"tagB", "tagC"}, {"tagE"}},
		},
		MetricRule{
			Namespace: "ns",
			Metrics: []MetricAttr{
				{Unit: "Bytes", Name: "fieldD"},
				{Unit: "Bytes", Name: "fieldA"},
				{Unit: "Bytes", Name: "fieldC"},
				{Unit: "Bytes", Name: "fieldB"},
			},
			DimensionSets: [][]string{{"tagC"}, {"tagA", "tagB"}, {"tagB"}},
		}}

	AttachMetricRuleWithDedup(m, rules)

	assert.True(t, m.HasField(MetricRuleKey))
	fields := m.Fields()[MetricRuleKey]
	filteredRule, _ := fields.([]MetricRule)
	assert.Equal(t, 3, len(filteredRule))

	expected := []struct {
		metrics    []MetricAttr
		dimensions [][]string
	}{
		{metrics: []MetricAttr{{Unit: "Bytes", Name: "fieldA"}},
			dimensions: [][]string{{"tagA", "tagB"}, {"tagB", "tagC"}, {"tagC"}, {"tagB"}},
		},
		{metrics: []MetricAttr{{Unit: "Bytes", Name: "fieldC"}},
			dimensions: [][]string{{"tagA", "tagB"}, {"tagB", "tagC"}, {"tagC"}, {"tagB"}},
		},
		{metrics: []MetricAttr{{Unit: "Bytes", Name: "fieldD"}, {Unit: "Bytes", Name: "fieldB"}},
			dimensions: [][]string{{"tagC"}, {"tagA", "tagB"}, {"tagB"}},
		},
	}

	for _, r := range filteredRule {
		found := false
		for _, v := range expected {
			if reflect.DeepEqual(r.Metrics, v.metrics) && reflect.DeepEqual(len(r.DimensionSets), len(v.dimensions)) {
				found = true
				break
			}
		}
		if !found {
			assert.Fail(t, fmt.Sprintf("Not Found Metrics:  \"%v\" ; DimensionSet: \"%v\" ", r.Metrics, r.DimensionSets))
		}
	}
}
