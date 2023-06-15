// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type MetricBatchSize struct {
}

func (m *MetricBatchSize) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("metric_batch_size", 1000, input)
	return
}

func init() {
	m := new(MetricBatchSize)
	RegisterRule("metric_batch_size", m)
}
