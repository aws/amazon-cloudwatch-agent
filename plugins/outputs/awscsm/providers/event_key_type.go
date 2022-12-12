// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package providers

import (
	"fmt"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/awscsm/csm"
)

var errEventEntryKeyTypeNil = newLoopControlError("metric key type is nil", loopStateContinue)

// EventEntryKeyType is a type alias of string that will
// allow for helper methods to check whether or not
// it is of a certain enum
type EventEntryKeyType string

// NewEventEntryKeyType return a new metric key type and will return a loop
// control error if k is nil.
func NewEventEntryKeyType(k *string) (EventEntryKeyType, error) {
	if k == nil {
		return EventEntryKeyType(""), errEventEntryKeyTypeNil
	}

	switch *k {
	case csm.MonitoringEventEntryKeyTypeNone:
		return EventEntryKeyType(*k), nil
	case csm.MonitoringEventEntryKeyTypeAggregation:
		return EventEntryKeyType(*k), nil
	case csm.MonitoringEventEntryKeyTypeAggregationTimestamp:
		return EventEntryKeyType(*k), nil
	case csm.MonitoringEventEntryKeyTypeSample:
		return EventEntryKeyType(*k), nil
	}

	return EventEntryKeyType(""), fmt.Errorf("invalid key type enum: %s", *k)
}

// IsNone will return true is if the metric is a none type
func (m EventEntryKeyType) IsNone() bool {
	return string(m) == csm.MonitoringEventEntryKeyTypeNone
}

// IsAggregation will return true is if the metric is a aggregation type
func (m EventEntryKeyType) IsAggregation() bool {
	return string(m) == csm.MonitoringEventEntryKeyTypeAggregation
}

// IsAggregationTimestamp will return true is if the metric is a aggregation timestamp type
func (m EventEntryKeyType) IsAggregationTimestamp() bool {
	return string(m) == csm.MonitoringEventEntryKeyTypeAggregationTimestamp
}

// IsSample will return true is if the metric is a sample type
func (m EventEntryKeyType) IsSample() bool {
	return string(m) == csm.MonitoringEventEntryKeyTypeSample
}
