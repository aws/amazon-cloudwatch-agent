// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emfProcessor

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/structuredlogscommon"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/processors"
)

type EmfProcessor struct {
	inited                  bool
	MetricDeclarationsDedup bool                 `toml:"metric_declaration_dedup"`
	MetricDeclarations      []*metricDeclaration `toml:"metric_declaration"`
	MetricNamespace         string               `toml:"metric_namespace"`
}

func (e *EmfProcessor) SampleConfig() string {
	return `
[[processors.emfProcessor]]

	metric_declaration_dedup = true
	metric_namespace = "ECS/ContainerInsights/Prometheus"
	[[processors.emfProcessor.metric_declaration]]
      dimensions = [["Service", "Namespace"]]
      labels_matcher = "my-nginx.*"
      labels_separator = ";"
      metric_selectors = ["^nginx_ingress_controller_requests$"]
      source_labels = ["Service"]
`
}

func (e *EmfProcessor) Description() string {
	return "EmfProcessor is used to filter emf log event and set emf"
}

func (e *EmfProcessor) Apply(in ...telegraf.Metric) (result []telegraf.Metric) {
	if !e.inited {
		for _, declaration := range e.MetricDeclarations {
			declaration.init()
		}
		e.inited = true
	}

	// Process each metric
	for _, metric := range in {
		tags := metric.Tags()
		fields := metric.Fields()

		var rules []structuredlogscommon.MetricRule
		// metric go through each MetricDeclaration filter to build MetricRules
		for _, declaration := range e.MetricDeclarations {
			retRule := declaration.process(tags, fields, e.MetricNamespace)
			if retRule != nil {
				rules = append(rules, *retRule)
			}
		}

		// set EMF according to calculated MetricRule
		if e.MetricDeclarationsDedup {
			structuredlogscommon.AttachMetricRuleWithDedup(metric, rules)
		} else {
			structuredlogscommon.AttachMetricRule(metric, rules)
		}

		result = append(result, metric)
	}
	return result
}

func init() {
	processors.Add("emfProcessor", func() telegraf.Processor {
		return &EmfProcessor{}
	})
}
