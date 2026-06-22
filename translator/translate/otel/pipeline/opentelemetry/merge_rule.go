// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
)

func init() {
	otelRule := mergeJsonUtil.NewSectionMergeRule("opentelemetry", "/")
	collectRule := mergeJsonUtil.NewSectionMergeRule("collect", otelRule.Path)
	otelRule.MergeMap[collectRule.SectionKey] = collectRule
	mergeJsonUtil.MergeRuleMap[otelRule.SectionKey] = otelRule
}
