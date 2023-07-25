// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecs

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected"
)

const SectionKey = "ecs"

type ECS struct {
}

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (e *ECS) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, SectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	e := new(ECS)
	parent.MergeRuleMap[SectionKey] = e
}
