// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prune

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
)

type MetricPruner struct {
}

func (p *MetricPruner) ShouldBeDropped(attributes pcommon.Map) (bool, error) {
	for _, attributeKey := range common.IndexableMetricAttributes {
		if val, ok := attributes.Get(attributeKey); ok {
			if !isAsciiPrintable(val.Str()) {
				return true, errors.New("Metric attribute " + attributeKey + " must contain only ASCII characters.")
			}
		}
	}
	return false, nil
}

func NewPruner() *MetricPruner {
	return &MetricPruner{}
}

func isAsciiPrintable(val string) bool {
	nonWhitespaceFound := false
	for _, c := range val {
		if c < 32 || c > 126 {
			return false
		} else if !nonWhitespaceFound && c != 32 {
			nonWhitespaceFound = true
		}
	}
	return nonWhitespaceFound
}
