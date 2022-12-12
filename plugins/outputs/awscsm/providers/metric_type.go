// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package providers

import (
	"fmt"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/awscsm/csm"
)

// MetricType is a type aliased string used to
// check for valid enums through utilit methods.
type MetricType string

var errMetricTypeNil = newLoopControlError("metric type is nil", loopStateContinue)

// NewMetricType will return a new metric type as long as t is
// not nil. If t is nil, a loop control error.
func NewMetricType(t *string) (MetricType, error) {
	if t == nil {
		return MetricType(""), errMetricTypeNil
	}

	switch *t {
	case csm.MonitoringEventEntryMetricTypeNone:
		return MetricType(*t), nil
	case csm.MonitoringEventEntryMetricTypeFrequency:
		return MetricType(*t), nil
	case csm.MonitoringEventEntryMetricTypeSeh:
		return MetricType(*t), nil
	}

	return MetricType(""), fmt.Errorf("invalid metric type enum: %s", *t)
}

// IsNone signifies a metric type that does not have any classification.
func (m MetricType) IsNone() bool {
	return m == csm.MonitoringEventEntryMetricTypeNone
}

// IsFrequency signifies a frequency metric.
func (m MetricType) IsFrequency() bool {
	return m == csm.MonitoringEventEntryMetricTypeFrequency
}

// IsSEH signifies a SEH metric.
func (m MetricType) IsSEH() bool {
	return m == csm.MonitoringEventEntryMetricTypeSeh
}
