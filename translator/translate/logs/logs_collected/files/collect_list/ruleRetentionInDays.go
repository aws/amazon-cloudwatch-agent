// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const RetentionInDaysSectionKey = "retention_in_days"

type RetentionInDays struct {
}

func (f *RetentionInDays) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultRetentionInDaysCase(RetentionInDaysSectionKey, float64(-1), input)
	returnKey = RetentionInDaysSectionKey
	return
}

func init() {
	l := new(RetentionInDays)
	r := []Rule{l}
	RegisterRule(RetentionInDaysSectionKey, r)
}
