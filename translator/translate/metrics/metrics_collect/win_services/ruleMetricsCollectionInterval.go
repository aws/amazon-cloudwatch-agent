// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package win_services

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

type MetricsCollectionInterval struct {
}

func (obj *MetricsCollectionInterval) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	return util.ProcessMetricsCollectionInterval(input, "", SectionKey)
}

func init() {
	obj := new(MetricsCollectionInterval)
	RegisterRule(util.Collect_Interval_Mapped_Key, obj)
}
