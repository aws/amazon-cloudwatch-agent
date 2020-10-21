// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emfprocessor

import "github.com/aws/amazon-cloudwatch-agent/translator"

const (
	SectionKeyMetricDeclarationDedup = "metric_declaration_dedup"
)

type MetricDeclarationDedup struct {
}

func (d *MetricDeclarationDedup) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKeyMetricDeclarationDedup, true, input)
	return
}

func init() {
	RegisterRule(SectionKeyMetricDeclarationDedup, new(MetricDeclarationDedup))
}
