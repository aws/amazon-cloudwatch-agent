// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

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

type LogFilter struct {
}

func (lf *LogFilter) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	var res []interface{}
	if val, ok := im[FiltersSectionKey]; ok {
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
		returnKey = FiltersSectionKey
	} else {
		returnKey = ""
	}
	returnVal = res
	return
}

func init() {
	lf := new(LogFilter)
	r := []Rule{lf}
	RegisterRule(FiltersSectionKey, r)
}
