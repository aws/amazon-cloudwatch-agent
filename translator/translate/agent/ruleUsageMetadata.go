// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
)

type ruleUsageMetadata struct {
}

const sectionKeyUsageMetadata = "usage_metadata"

var _ mergeJsonRule.MergeRule = (*ruleUsageMetadata)(nil)

func (r *ruleUsageMetadata) Merge(source map[string]any, result map[string]any) {
	mergeJsonUtil.MergeList(source, result, sectionKeyUsageMetadata)
}

func init() {
	r := new(ruleUsageMetadata)
	MergeRuleMap[sectionKeyUsageMetadata] = r
}
