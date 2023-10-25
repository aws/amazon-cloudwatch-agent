// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package customconfiguration

import "go.opentelemetry.io/collector/pdata/pcommon"

type KeepActions struct {
	Actions []ActionItem
}

func NewCustomKeeper(rules []Rule) *KeepActions {
	return &KeepActions{
		Actions: generateActionDetails(rules, "keep"),
	}
}

func (k *KeepActions) ShouldBeDropped(attributes, _ pcommon.Map) (bool, error) {
	// nothing will be dropped if no keep rule is defined
	if k.Actions == nil || len(k.Actions) == 0 {
		return false, nil
	}
	for _, element := range k.Actions {
		isMatched, err := isSelected(attributes, element.SelectorMatchers, false)
		if isMatched {
			// The data point will not be dropped if one of keep rule matched
			return false, nil
		}
		if err != nil {
			// The data point will be dropped when error is found
			return true, err
		}
	}
	return true, nil
}
