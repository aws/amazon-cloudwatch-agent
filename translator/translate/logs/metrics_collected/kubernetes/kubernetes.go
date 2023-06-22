// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package kubernetes

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/metrics_collected"
)

const SectionKey = "kubernetes"

type Kubernetes struct {
}

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (k *Kubernetes) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeMap(source, result, SectionKey, MergeRuleMap, GetCurPath())
}

func init() {
	k := new(Kubernetes)
	parent.MergeRuleMap[SectionKey] = k
}
