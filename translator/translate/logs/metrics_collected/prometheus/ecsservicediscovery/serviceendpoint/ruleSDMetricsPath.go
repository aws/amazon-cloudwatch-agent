// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package serviceendpoint

const (
	SectionKeySDMetricsPath = "sd_metrics_path"
)

type SDMetricsPath struct {
}

// Optional Key
func (d *SDMetricsPath) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	if val, ok := im[SectionKeySDMetricsPath]; !ok {
		returnKey = ""
		returnVal = ""

	} else {
		returnKey = SectionKeySDMetricsPath
		returnVal = val
	}
	return
}

func init() {
	RegisterRule(SectionKeySDMetricsPath, new(SDMetricsPath))
}
