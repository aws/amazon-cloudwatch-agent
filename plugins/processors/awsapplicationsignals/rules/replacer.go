// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rules

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
)

type ReplaceActions struct {
	Actions                 []ActionItem
	markDataPointAsReserved bool
}

func NewReplacer(rules []Rule, markDataPointAsReserved bool) *ReplaceActions {
	return &ReplaceActions{
		Actions:                 generateActionDetails(rules, AllowListActionReplace),
		markDataPointAsReserved: markDataPointAsReserved,
	}
}

func (r *ReplaceActions) Process(attributes, _ pcommon.Map, isTrace bool) error {
	// do nothing when there is no replace rule defined
	if r.Actions == nil || len(r.Actions) == 0 {
		return nil
	}
	// If there are more than one rule are matched, the last one will be executed(Later one has higher priority)
	actions := r.Actions
	finalRules := make(map[string]string)
	for i := len(actions) - 1; i >= 0; i = i - 1 {
		element := actions[i]
		isMatched := matchesSelectors(attributes, element.SelectorMatchers, isTrace)
		if !isMatched {
			continue
		}
		for _, replacement := range element.Replacements {
			targetDimension := replacement.TargetDimension

			attr := convertToManagedAttributeKey(targetDimension, isTrace)
			// every replacement in one specific dimension only will be performed once
			if _, visited := finalRules[attr]; !visited {
				finalRules[attr] = replacement.Value
			}
		}
	}

	for key, value := range finalRules {
		attributes.PutStr(key, value)
	}

	if len(finalRules) > 0 && r.markDataPointAsReserved {
		attributes.PutBool(common.AttributeTmpReserved, true)
	}
	return nil
}
