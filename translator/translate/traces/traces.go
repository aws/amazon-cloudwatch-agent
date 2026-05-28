// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package traces

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
)

func init() {
	tracesRule := mergeJsonUtil.NewSectionMergeRule("traces", "/")
	collectRule := mergeJsonUtil.NewSectionMergeRule("traces_collected", tracesRule.Path)
	tracesRule.MergeMap[collectRule.SectionKey] = collectRule
	mergeJsonUtil.MergeRuleMap[tracesRule.SectionKey] = tracesRule
}
