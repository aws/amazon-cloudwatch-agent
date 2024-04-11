// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsapplicationsignals

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/rules"
)

var testRules = []rules.Rule{
	{
		Selectors: []rules.Selector{
			{
				Dimension: "dim_action",
				Match:     "reserved",
			},
			{
				Dimension: "dim_val",
				Match:     "test1",
			},
		},
		Replacements: []rules.Replacement{
			{
				TargetDimension: "dim_val",
				Value:           "test2",
			},
		},
		Action: "replace",
	},
	{
		Selectors: []rules.Selector{
			{
				Dimension: "dim_action",
				Match:     "reserved",
			},
		},
		Action: "keep",
	},
	{
		Selectors: []rules.Selector{
			{
				Dimension: "dim_drop",
				Match:     "hc",
			},
		},
		Action: "drop",
	},
}

func TestProcessMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ap := &awsapplicationsignalsprocessor{
		logger: logger,
		config: &config.Config{
			Resolvers: []config.Resolver{config.NewGenericResolver("")},
			Rules:     testRules,
		},
	}

	ctx := context.Background()
	ap.StartMetrics(ctx, nil)

	keepMetrics := generateMetrics(map[string]string{
		"dim_action": "reserved",
		"dim_val":    "test",
		"dim_op":     "keep",
	})
	ap.processMetrics(ctx, keepMetrics)
	assert.Equal(t, "reserved", getDimensionValue(t, keepMetrics, "dim_action"))
	assert.Equal(t, "test", getDimensionValue(t, keepMetrics, "dim_val"))

	replaceMetrics := generateMetrics(map[string]string{
		"dim_action": "reserved",
		"dim_val":    "test1",
	})
	ap.processMetrics(ctx, replaceMetrics)
	assert.Equal(t, "reserved", getDimensionValue(t, replaceMetrics, "dim_action"))
	assert.Equal(t, "test2", getDimensionValue(t, replaceMetrics, "dim_val"))

	dropMetricsByDrop := generateMetrics(map[string]string{
		"dim_action": "reserved",
		"dim_drop":   "hc",
	})
	ap.processMetrics(ctx, dropMetricsByDrop)
	assert.True(t, isMetricNil(dropMetricsByDrop))

	dropMetricsByKeep := generateMetrics(map[string]string{
		"dim_op": "drop",
	})
	ap.processMetrics(ctx, dropMetricsByKeep)
	assert.True(t, isMetricNil(dropMetricsByKeep))
}

func TestProcessMetricsLowercase(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ap := &awsapplicationsignalsprocessor{
		logger: logger,
		config: &config.Config{
			Resolvers: []config.Resolver{config.NewGenericResolver("")},
			Rules:     testRules,
		},
	}

	ctx := context.Background()
	ap.StartMetrics(ctx, nil)

	lowercaseMetrics := pmetric.NewMetrics()
	errorMetric := lowercaseMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	errorMetric.SetName("error")
	latencyMetric := lowercaseMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	latencyMetric.SetName("latency")
	faultMetric := lowercaseMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	faultMetric.SetName("fault")

	ap.processMetrics(ctx, lowercaseMetrics)
	assert.Equal(t, "Error", lowercaseMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Name())
	assert.Equal(t, "Latency", lowercaseMetrics.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(0).Name())
	assert.Equal(t, "Fault", lowercaseMetrics.ResourceMetrics().At(2).ScopeMetrics().At(0).Metrics().At(0).Name())
}

func TestProcessTraces(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ap := &awsapplicationsignalsprocessor{
		logger: logger,
		config: &config.Config{
			Resolvers: []config.Resolver{config.NewGenericResolver("")},
			Rules:     testRules,
		},
	}

	ctx := context.Background()
	ap.StartTraces(ctx, nil)

	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("dim_action", "reserved")
	span.Attributes().PutStr("dim_val", "test1")

	ap.processTraces(ctx, traces)

	actualSpan := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	actualVal, _ := actualSpan.Attributes().Get("dim_val")
	assert.Equal(t, "test2", actualVal.AsString())
}

func generateMetrics(dimensions map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()

	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	gauge := m.SetEmptyGauge().DataPoints().AppendEmpty()
	gauge.SetIntValue(10)

	m = md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	sum := m.SetEmptySum().DataPoints().AppendEmpty()
	sum.SetIntValue(10)

	m = md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	expoHistogram := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
	expoHistogram.SetSum(10)

	m = md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	summary := m.SetEmptySummary().DataPoints().AppendEmpty()
	summary.SetSum(10)

	m = md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	histogram := m.SetEmptyHistogram().DataPoints().AppendEmpty()
	histogram.SetSum(10)

	for k, v := range dimensions {
		gauge.Attributes().PutStr(k, v)
		sum.Attributes().PutStr(k, v)
		expoHistogram.Attributes().PutStr(k, v)
		summary.Attributes().PutStr(k, v)
		histogram.Attributes().PutStr(k, v)
	}

	return md
}

func getDimensionValue(t *testing.T, m pmetric.Metrics, dimensionName string) string {
	var agreedValue string

	gauge := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge()
	if val, ok := gauge.DataPoints().At(0).Attributes().Get(dimensionName); !ok {
		t.Errorf("no dimension value is found with key %s\n", dimensionName)
	} else {
		agreedValue = val.AsString()
	}

	sum := m.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(0).Sum()
	if val, ok := sum.DataPoints().At(0).Attributes().Get(dimensionName); !ok {
		t.Errorf("no dimension value is found with key %s\n", dimensionName)
	} else {
		newVal := val.AsString()
		if agreedValue != newVal {
			t.Errorf("inconsistent dimension value, agreed value is %s, new %s\n", agreedValue, newVal)
		}
	}

	expoHistogram := m.ResourceMetrics().At(2).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram()
	if val, ok := expoHistogram.DataPoints().At(0).Attributes().Get(dimensionName); !ok {
		t.Errorf("no dimension value is found with key %s\n", dimensionName)
	} else {
		newVal := val.AsString()
		if agreedValue != newVal {
			t.Errorf("inconsistent dimension value, agreed value is %s, new %s\n", agreedValue, newVal)
		}
	}

	summary := m.ResourceMetrics().At(3).ScopeMetrics().At(0).Metrics().At(0).Summary()
	if val, ok := summary.DataPoints().At(0).Attributes().Get(dimensionName); !ok {
		t.Errorf("no dimension value is found with key %s\n", dimensionName)
	} else {
		newVal := val.AsString()
		if agreedValue != newVal {
			t.Errorf("inconsistent dimension value, agreed value is %s, new %s\n", agreedValue, newVal)
		}
	}

	histogram := m.ResourceMetrics().At(4).ScopeMetrics().At(0).Metrics().At(0).Histogram()
	if val, ok := histogram.DataPoints().At(0).Attributes().Get(dimensionName); !ok {
		t.Errorf("no dimension value is found with key %s\n", dimensionName)
	} else {
		newVal := val.AsString()
		if agreedValue != newVal {
			t.Errorf("inconsistent dimension value, agreed value is %s, new %s\n", agreedValue, newVal)
		}
	}
	return agreedValue
}

func isMetricNil(m pmetric.Metrics) bool {
	gauge := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()
	if gauge.Len() > 0 {
		return false
	}

	sum := m.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints()
	if sum.Len() > 0 {
		return false
	}

	expoHistogram := m.ResourceMetrics().At(2).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints()
	if expoHistogram.Len() > 0 {
		return false
	}

	summary := m.ResourceMetrics().At(3).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints()
	if summary.Len() > 0 {
		return false
	}

	histogram := m.ResourceMetrics().At(4).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints()
	if histogram.Len() > 0 {
		return false
	}
	return true
}
