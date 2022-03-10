// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emfProcessor

import (
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/structuredlogscommon"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/influxdata/telegraf/testutil"
)

func buildTestMetricDeclarations() (mds []*metricDeclaration) {
	md1 := &metricDeclaration{
		SourceLabels:    []string{"tagA"},
		LabelMatcher:    "^v1$",
		MetricSelectors: []string{"metric_a"},
		Dimensions:      [][]string{{"tagA"}},
	}
	mds = append(mds, md1)

	md2 := &metricDeclaration{
		SourceLabels:    []string{"tagA"},
		LabelMatcher:    "^v1$",
		MetricSelectors: []string{"metric_a", "metric_d"},
		Dimensions:      [][]string{{"tagA"}},
	}
	mds = append(mds, md2)
	return
}

func buildMetricUnit() (mu map[string]string) {
	return map[string]string{
		"metric_a": "Count",
		"metric_b": "Percent",
		"metric_d": "Megabytes",
	}
}

func buildTestMetrics(ts time.Time) (ms []telegraf.Metric) {
	m1 := metric.New("prometheus_scraper",
		map[string]string{"tagA": "v1"},
		map[string]interface{}{"metric_a": 0.1},
		ts)
	ms = append(ms, m1)
	return
}

func buildEMFMetricRule() *structuredlogscommon.MetricRule {
	return &structuredlogscommon.MetricRule{
		Namespace:     "ContainerInsights/Prometheus",
		DimensionSets: [][]string{{"tagA"}},
		Metrics:       []structuredlogscommon.MetricAttr{{Name: "metric_a"}},
	}
}

func buildEMFMetricRuleWithUnit() *structuredlogscommon.MetricRule {
	return &structuredlogscommon.MetricRule{
		Namespace:     "ContainerInsights/Prometheus",
		DimensionSets: [][]string{{"tagA"}},
		Metrics:       []structuredlogscommon.MetricAttr{{Name: "metric_a", Unit: "Count"}},
	}
}

func buildExpectedMetrics(ts time.Time) (ms []telegraf.Metric) {
	m1 := metric.New("prometheus_scraper",
		map[string]string{"tagA": "v1", "attributesInFields": "CloudWatchMetrics"},
		map[string]interface{}{"metric_a": 0.1, "CloudWatchMetrics": []structuredlogscommon.MetricRule{*buildEMFMetricRule(), *buildEMFMetricRule()}},
		ts)
	ms = append(ms, m1)
	return
}

func buildExpectedDeduppedMetrics(ts time.Time) (ms []telegraf.Metric) {
	m1 := metric.New("prometheus_scraper",
		map[string]string{"tagA": "v1", "attributesInFields": "CloudWatchMetrics"},
		map[string]interface{}{"metric_a": 0.1, "CloudWatchMetrics": []structuredlogscommon.MetricRule{*buildEMFMetricRule()}},
		ts)
	ms = append(ms, m1)
	return
}

func buildExpectedMetricsWithUnit(ts time.Time) (ms []telegraf.Metric) {
	m1 := metric.New("prometheus_scraper",
		map[string]string{"tagA": "v1", "attributesInFields": "CloudWatchMetrics"},
		map[string]interface{}{"metric_a": 0.1, "CloudWatchMetrics": []structuredlogscommon.MetricRule{*buildEMFMetricRuleWithUnit(), *buildEMFMetricRuleWithUnit()}},
		ts)
	ms = append(ms, m1)
	return
}

func buildExpectedDeduppedMetricsWithUnit(ts time.Time) (ms []telegraf.Metric) {
	m1 := metric.New("prometheus_scraper",
		map[string]string{"tagA": "v1", "attributesInFields": "CloudWatchMetrics"},
		map[string]interface{}{"metric_a": 0.1, "CloudWatchMetrics": []structuredlogscommon.MetricRule{*buildEMFMetricRuleWithUnit()}},
		ts)
	ms = append(ms, m1)
	return
}

func TestEmfProcessor_Apply(t *testing.T) {
	type fields struct {
		inited                  bool
		MetricDeclarationsDedup bool
		MetricDeclarations      []*metricDeclaration
	}
	type args struct {
		in []telegraf.Metric
	}

	ts := time.Now()
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantResult []telegraf.Metric
	}{
		{name: "dedupped",
			fields: fields{MetricDeclarationsDedup: true,
				MetricDeclarations: buildTestMetricDeclarations()},
			args:       args{in: buildTestMetrics(ts)},
			wantResult: buildExpectedDeduppedMetrics(ts),
		},
		{name: "not_dedupped",
			fields: fields{MetricDeclarationsDedup: false,
				MetricDeclarations: buildTestMetricDeclarations()},
			args:       args{in: buildTestMetrics(ts)},
			wantResult: buildExpectedMetrics(ts),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EmfProcessor{
				inited:                  tt.fields.inited,
				MetricDeclarationsDedup: tt.fields.MetricDeclarationsDedup,
				MetricDeclarations:      tt.fields.MetricDeclarations,
				MetricNamespace:         "ContainerInsights/Prometheus",
			}

			gotResult := e.Apply(tt.args.in...)
			testutil.RequireMetricsEqual(t, tt.wantResult, gotResult)
		})
	}
}

func TestEmfProcessor_Apply_WithMetricUnit(t *testing.T) {
	type fields struct {
		inited                  bool
		MetricDeclarationsDedup bool
		MetricDeclarations      []*metricDeclaration
		MetricUnit              map[string]string
	}
	type args struct {
		in []telegraf.Metric
	}

	ts := time.Now()
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantResult []telegraf.Metric
	}{
		{name: "dedupped",
			fields: fields{MetricDeclarationsDedup: true,
				MetricDeclarations: buildTestMetricDeclarations(),
				MetricUnit:         buildMetricUnit()},
			args:       args{in: buildTestMetrics(ts)},
			wantResult: buildExpectedDeduppedMetricsWithUnit(ts),
		},
		{name: "not_dedupped",
			fields: fields{MetricDeclarationsDedup: false,
				MetricDeclarations: buildTestMetricDeclarations(),
				MetricUnit:         buildMetricUnit()},
			args:       args{in: buildTestMetrics(ts)},
			wantResult: buildExpectedMetricsWithUnit(ts),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EmfProcessor{
				inited:                  tt.fields.inited,
				MetricDeclarationsDedup: tt.fields.MetricDeclarationsDedup,
				MetricDeclarations:      tt.fields.MetricDeclarations,
				MetricNamespace:         "ContainerInsights/Prometheus",
				MetricUnit:              tt.fields.MetricUnit,
			}

			gotResult := e.Apply(tt.args.in...)
			testutil.RequireMetricsEqual(t, tt.wantResult, gotResult)
		})
	}
}
