// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emfProcessor

import (
	"bytes"
	"regexp"
	"sort"

	"github.com/aws/amazon-cloudwatch-agent/internal/structuredlogscommon"
)

// Declare metric matcher to match metrics with {source_labels, label_separator, labels_matcher and metric_selectors},
// When a metric have all the source_labels concatenated with label_separator matching the labels_matcher,
// and it is also matched by any of metrics_selectors, it will be set with EMF based on the customized "dimensions"
type metricDeclaration struct {
	SourceLabels    []string   `toml:"source_labels"`
	LabelSeparator  string     `toml:"label_separator"`
	LabelMatcher    string     `toml:"label_matcher"`
	MetricSelectors []string   `toml:"metric_selectors"`
	MetricNamespace string     `toml:"metric_namespace"`
	Dimensions      [][]string `toml:"dimensions"`
	regexP        *regexp.Regexp
	metricRegexPs []*regexp.Regexp
}

// calculate EMF MetricRule for each metric's tags and fields
func (m *metricDeclaration) process(tags map[string]string, fields map[string]interface{}, namespace string, metricUnit map[string]string) (resRule *structuredlogscommon.MetricRule) {
	// If there is no source_labels or metric_selectors defined, the metricDeclaration is not valid
	if len(m.SourceLabels) == 0 || len(m.MetricSelectors) == 0 {
		return
	}

	// Set destination namespace to send prometheus metrics from each job
	destinationNamespace := namespace

	if m.MetricNamespace !="" {
		destinationNamespace = m.MetricNamespace
	}
	// get concatenated source_labels
	concatenatedLabels := m.getConcatenatedLabels(tags)

	// try match the source_labels
	if !m.regexP.MatchString(concatenatedLabels) {
		return
	}

	rule := &structuredlogscommon.MetricRule{Namespace: destinationNamespace}

	// For metric matching the labels_matcher, try match its fields with metric_selectors
Loop:
	for fieldKey := range fields {
		for _, regexP := range m.metricRegexPs {
			if regexP.MatchString(fieldKey) {
				if unit, ok := metricUnit[fieldKey]; ok {
					rule.Metrics = append(rule.Metrics, structuredlogscommon.MetricAttr{Name: fieldKey, Unit: unit})
					continue Loop
				}

				rule.Metrics = append(rule.Metrics, structuredlogscommon.MetricAttr{Name: fieldKey})
				continue Loop
			}
		}
	}

	// return the valid result MetricRule for the matching metric
	if len(rule.Metrics) > 0 {
		rule.DimensionSets = m.Dimensions
		resRule = rule
	}
	return
}

// get source_labels concatenated with LabelSeparator
func (m *metricDeclaration) getConcatenatedLabels(tags map[string]string) (result string) {
	concatenatedLabelBuf := new(bytes.Buffer)
	isFirstLabel := true
	for _, sourceLabel := range m.SourceLabels {
		if isFirstLabel {
			isFirstLabel = false
		} else {
			concatenatedLabelBuf.WriteString(m.LabelSeparator)
		}

		concatenatedLabelBuf.WriteString(tags[sourceLabel])
	}
	return concatenatedLabelBuf.String()
}

func (m *metricDeclaration) init() {
	m.regexP = regexp.MustCompile(m.LabelMatcher)
	if m.LabelSeparator == "" {
		m.LabelSeparator = ";"
	}

	for _, dim := range m.Dimensions {
		sort.Strings(dim)
	}

	for _, metricRegex := range m.MetricSelectors {
		m.metricRegexPs = append(m.metricRegexPs, regexp.MustCompile(metricRegex))
	}
}
