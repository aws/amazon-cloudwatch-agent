// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package customconfiguration

import "go.opentelemetry.io/collector/pdata/pcommon"

type DropActions struct {
	Actions []ActionItem
}

func NewCustomDropper(rules []Rule) *DropActions {
	return &DropActions{
		Actions: generateActionDetails(rules, "drop"),
	}
}

func (d *DropActions) ShouldBeDropped(attributes, _ pcommon.Map) (bool, error) {
	// nothing will be dropped if no rule is defined
	if d.Actions == nil || len(d.Actions) == 0 {
		return false, nil
	}
	for _, element := range d.Actions {
		isMatched, err := isSelected(attributes, element.SelectorMatchers, false)
		if isMatched {
			// The datapoint will be dropped if one of keep rule matched
			return true, nil
		}
		if err != nil {
			// The datapoint will be kept if error is occurred in match process
			return false, err
		}
	}
	return false, nil
}
