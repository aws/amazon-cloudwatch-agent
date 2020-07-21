// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type MetricBufferLimit struct {
}

func (m *MetricBufferLimit) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("metric_buffer_limit", 10000, input)
	return
}

func init() {
	m := new(MetricBufferLimit)
	RegisterRule("metric_buffer_limit", m)
}
