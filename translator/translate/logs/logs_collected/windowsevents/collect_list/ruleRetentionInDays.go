// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const RetentionInDaysSectionKey = "retention_in_days"

type RetentionInDays struct {
}

func (f *RetentionInDays) ApplyRule(input interface{}) (string, interface{}) {
	var returnVal interface{}
	_, returnVal = translator.DefaultRetentionInDaysCase(RetentionInDaysSectionKey, float64(-1), input)
	returnKey := RetentionInDaysSectionKey
	return returnKey, returnVal
}

func init() {
	l := new(RetentionInDays)
	RegisterRule(RetentionInDaysSectionKey, l)
}
