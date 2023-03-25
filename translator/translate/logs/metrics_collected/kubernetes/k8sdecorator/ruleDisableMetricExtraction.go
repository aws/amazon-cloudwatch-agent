// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sdecorator

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const (
	SectionKeyDisableMetricExtraction = "disable_metric_extraction"
)

type DisableMetricExtraction struct {
}

func (t *DisableMetricExtraction) ApplyRule(input interface{}) (string, interface{}) {
	return translator.DefaultCase(SectionKeyDisableMetricExtraction, false, input)
}

func init() {
	RegisterRule(SectionKeyDisableMetricExtraction, new(DisableMetricExtraction))
}
