// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rules

import "go.opentelemetry.io/collector/pdata/pcommon"

type DropActions struct {
	Actions []ActionItem
}

func NewDropper(rules []Rule) *DropActions {
	return &DropActions{
		Actions: generateActionDetails(rules, AllowListActionDrop),
	}
}

func (d *DropActions) ShouldBeDropped(attributes pcommon.Map) (bool, error) {
	// nothing will be dropped if no rule is defined
	if d.Actions == nil || len(d.Actions) == 0 {
		return false, nil
	}
	for _, element := range d.Actions {
		isMatched := matchesSelectors(attributes, element.SelectorMatchers, false)
		if isMatched {
			// drop the datapoint as one of drop rules is matched
			return true, nil
		}
	}
	return false, nil
}
