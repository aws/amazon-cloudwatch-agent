// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

import (
	"fmt"
	"regexp"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const (
	FiltersSectionKey           = "filters"
	FiltersTypeSectionKey       = "type"
	FiltersExpressionSectionKey = "expression"
)

// WindowsEventFilter handles regex-based filtering for Windows event logs.
// Supports "include" and "exclude" filter types with regex expressions.
type WindowsEventFilter struct {
}

func (wef *WindowsEventFilter) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	val, ok := im[FiltersSectionKey]
	if !ok {
		return "", []interface{}{}
	}

	var res []interface{}
	filterArr := val.([]interface{})
	for _, filter := range filterArr {
		filterMap := map[string]interface{}{}

		_, filterVal := translator.DefaultCase(FiltersTypeSectionKey, "", filter)
		if filterVal == "" {
			translator.AddErrorMessages(GetCurPath()+FiltersSectionKey, fmt.Sprintf("Filter %s is invalid", filter))
			continue
		}
		filterMap[FiltersTypeSectionKey] = filterVal
		_, filterVal = translator.DefaultCase(FiltersExpressionSectionKey, "", filter)
		if filterVal == "" {
			translator.AddErrorMessages(GetCurPath()+FiltersSectionKey, fmt.Sprintf("Filter %s is invalid", filter))
			continue
		}
		if _, err := regexp.Compile(filterVal.(string)); err != nil {
			translator.AddErrorMessages(GetCurPath()+FiltersSectionKey, fmt.Sprintf("Filter expression %s is invalid", filter))
			continue
		}
		filterMap[FiltersExpressionSectionKey] = filterVal
		res = append(res, filterMap)
	}
	return FiltersSectionKey, res
}

func init() {
	r := new(WindowsEventFilter)
	RegisterRule(FiltersSectionKey, r)
}
