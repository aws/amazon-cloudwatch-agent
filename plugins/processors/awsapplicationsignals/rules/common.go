// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rules

import (
	"errors"

	"github.com/gobwas/glob"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/common"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/internal/attributes"
)

type AllowListAction string

const (
	AllowListActionKeep    AllowListAction = "keep"
	AllowListActionDrop    AllowListAction = "drop"
	AllowListActionReplace AllowListAction = "replace"
)

type Selector struct {
	Dimension string `mapstructure:"dimension"`
	Match     string `mapstructure:"match"`
}

type Replacement struct {
	TargetDimension string `mapstructure:"target_dimension"`
	Value           string `mapstructure:"value"`
}

type Rule struct {
	Selectors    []Selector      `mapstructure:"selectors"`
	Replacements []Replacement   `mapstructure:"replacements,omitempty"`
	Action       AllowListAction `mapstructure:"action"`
	RuleName     string          `mapstructure:"rule_name,omitempty"`
}

type SelectorMatcherItem struct {
	Key     string
	Matcher glob.Glob
}

type ActionItem struct {
	SelectorMatchers []SelectorMatcherItem
	Replacements     []Replacement `mapstructure:",omitempty"`
}

var traceKeyMap = map[string]string{
	common.MetricAttributeLocalService:             attributes.AWSLocalService,
	common.MetricAttributeEnvironment:              attributes.AWSLocalEnvironment,
	common.MetricAttributeLocalOperation:           attributes.AWSLocalOperation,
	common.MetricAttributeRemoteService:            attributes.AWSRemoteService,
	common.MetricAttributeRemoteEnvironment:        attributes.AWSRemoteEnvironment,
	common.MetricAttributeRemoteOperation:          attributes.AWSRemoteOperation,
	common.MetricAttributeRemoteResourceIdentifier: attributes.AWSRemoteResourceIdentifier,
	common.MetricAttributeRemoteResourceType:       attributes.AWSRemoteResourceType,
}

func GetAllowListAction(action string) (AllowListAction, error) {
	switch action {
	case "drop":
		return AllowListActionDrop, nil
	case "keep":
		return AllowListActionKeep, nil
	case "replace":
		return AllowListActionReplace, nil
	}
	return "", errors.New("invalid action in rule")
}

func convertToManagedAttributeKey(attributeKey string, isTrace bool) string {
	val, ok := traceKeyMap[attributeKey]
	if ok && isTrace {
		return val
	}
	return attributeKey
}

func matchesSelectors(attributes pcommon.Map, selectorMatchers []SelectorMatcherItem, isTrace bool) bool {
	for _, item := range selectorMatchers {
		exactKey := convertToManagedAttributeKey(item.Key, isTrace)
		value, ok := attributes.Get(exactKey)
		if !ok {
			return false
		}
		if !item.Matcher.Match(value.AsString()) {
			return false
		}
	}
	return true
}

func generateSelectorMatchers(selectors []Selector) []SelectorMatcherItem {
	var selectorMatchers []SelectorMatcherItem
	for _, selector := range selectors {
		selectorMatcherItem := SelectorMatcherItem{
			selector.Dimension,
			glob.MustCompile(selector.Match),
		}
		selectorMatchers = append(selectorMatchers, selectorMatcherItem)
	}
	return selectorMatchers
}

func generateActionDetails(rules []Rule, action AllowListAction) []ActionItem {
	var actionItems []ActionItem
	for _, rule := range rules {
		if rule.Action == action {
			var selectorMatchers = generateSelectorMatchers(rule.Selectors)
			actionItem := ActionItem{
				selectorMatchers,
				rule.Replacements,
			}
			actionItems = append(actionItems, actionItem)
		}
	}

	return actionItems
}
