// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ethtool

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type MetricsInclude struct {
}

const SectionKey_MetricsInclude = "metrics_include"

func (obj *MetricsInclude) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase(SectionKey_MetricsInclude, []string{}, input)
	returnKey = "fieldpass"
	return
}

func init() {
	obj := new(MetricsInclude)
	RegisterRule(SectionKey_MetricsInclude, obj)
}
