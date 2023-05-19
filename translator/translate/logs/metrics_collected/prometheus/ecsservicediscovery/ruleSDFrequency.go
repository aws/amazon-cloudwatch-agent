// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsservicediscovery

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

const (
	SectionKeySDFrequency = "sd_frequency"
)

type SDFrequency struct {
}

func (d *SDFrequency) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKeySDFrequency, "1m", input)
	return
}

func init() {
	RegisterRule(SectionKeySDFrequency, new(SDFrequency))
}
