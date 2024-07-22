// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rules

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
)

type KeepActions struct {
	Actions                 []ActionItem
	markDataPointAsReserved bool
}

func NewKeeper(rules []Rule, markDataPointAsReserved bool) *KeepActions {
	return &KeepActions{
		Actions:                 generateActionDetails(rules, AllowListActionKeep),
		markDataPointAsReserved: markDataPointAsReserved,
	}
}

func (k *KeepActions) ShouldBeDropped(attributes pcommon.Map) (bool, error) {
	// nothing will be dropped if no keep rule is defined
	if k.Actions == nil || len(k.Actions) == 0 {
		return false, nil
	}
	for _, element := range k.Actions {
		isMatched := matchesSelectors(attributes, element.SelectorMatchers, false)
		if k.markDataPointAsReserved {
			attributes.PutBool(common.AttributeTmpReserved, true)
		}
		if isMatched {
			// keep the datapoint as one of the keep rules is matched
			return false, nil
		}
	}
	return true, nil
}
