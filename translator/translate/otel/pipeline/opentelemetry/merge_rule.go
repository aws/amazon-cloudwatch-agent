// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
)

type mergeRule struct {
	sectionKey string
	path       string
	mergeMap   map[string]mergeJsonRule.MergeRule
}

func newMergeRule(sectionKey string, parentPath string) *mergeRule {
	return &mergeRule{
		sectionKey: sectionKey,
		path:       fmt.Sprintf("%s%s/", parentPath, sectionKey),
		mergeMap:   make(map[string]mergeJsonRule.MergeRule),
	}
}

func (m mergeRule) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, m.sectionKey, m.mergeMap, m.path)
}

func init() {
	otelRule := newMergeRule("opentelemetry", "/")
	collectRule := newMergeRule("collect", otelRule.path)
	otelRule.mergeMap[collectRule.sectionKey] = collectRule

	mergeJsonUtil.MergeRuleMap[otelRule.sectionKey] = otelRule
}
