// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package profiles

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
)

func init() {
	profilesRule := mergeJsonUtil.NewSectionMergeRule("profiles", "/")
	mergeJsonUtil.MergeRuleMap[profilesRule.SectionKey] = profilesRule
}
