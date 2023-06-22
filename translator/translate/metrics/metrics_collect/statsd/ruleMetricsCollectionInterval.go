// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statsd

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/util"
)

type MetricsCollectionInterval struct {
}

func (obj *MetricsCollectionInterval) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	return util.ProcessMetricsCollectionInterval(input, "10s", SectionKey)
}

func init() {
	obj := new(MetricsCollectionInterval)
	RegisterRule(util.Collect_Interval_Mapped_Key, obj)
}
