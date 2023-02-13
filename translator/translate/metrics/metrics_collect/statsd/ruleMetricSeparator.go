// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statsd

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type MetricSeparator struct {
}

const SectionKey_MetricSeparator = "metric_separator"

func (obj *MetricSeparator) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	key, val := translator.DefaultCase(SectionKey_MetricSeparator, "", input)
	if val != "" {
		return key, val
	}
	return
}

func init() {
	obj := new(MetricSeparator)
	RegisterRule(SectionKey_MetricSeparator, obj)
}
