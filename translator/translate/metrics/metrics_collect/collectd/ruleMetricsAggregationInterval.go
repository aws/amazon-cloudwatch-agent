// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collected

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/util"
)

type MetricsAggregationInterval struct {
}

const SectionKey_MetricsAggregationInterval = "metrics_aggregation_interval"

func (obj *MetricsAggregationInterval) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	return util.ProcessMetricsAggregationInterval(input, "60s", SectionKey)
}

func init() {
	obj := new(MetricsAggregationInterval)
	RegisterRule(SectionKey_MetricsAggregationInterval, obj)
}
