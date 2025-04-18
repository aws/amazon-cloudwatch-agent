// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrichandlers

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
)

type Pruner struct {
}

func (p *Pruner) ShouldBeDropped(attributes pcommon.Map) (bool, error) {
	for _, attributeKey := range common.CWMetricAttributes {
		if val, ok := attributes.Get(attributeKey); ok {
			if !isAsciiPrintable(val.Str()) {
				return true, errors.New("Metric attribute " + attributeKey + " must contain only ASCII characters.")
			}
		}
		if _, ok := attributes.Get(common.MetricAttributeTelemetrySource); !ok {
			return true, errors.New(fmt.Sprintf("Metric must contain %s.", common.MetricAttributeTelemetrySource))
		}
	}
	return false, nil
}

func NewPruner() *Pruner {
	return &Pruner{}
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
