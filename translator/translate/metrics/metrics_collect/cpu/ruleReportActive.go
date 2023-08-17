// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cpu

import (
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

type ReportActive struct {
}

func (r *ReportActive) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	measurementNames := util.GetMeasurementName(input)
	for _, measurementName := range measurementNames {
		if strings.HasSuffix(measurementName, "active") {
			returnKey = "report_active"
			returnVal = true
		}
	}
	return
}

func init() {
	r := new(ReportActive)
	RegisterRule("report_active", r)
}
