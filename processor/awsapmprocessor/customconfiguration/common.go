// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package customconfiguration

import (
	"fmt"

	"github.com/gobwas/glob"
	"go.opentelemetry.io/collector/pdata/pcommon"
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
	Selectors    []Selector    `mapstructure:"selectors"`
	Replacements []Replacement `mapstructure:"replacements,omitempty"`
	Action       string        `mapstructure:"action"`
	RuleName     string        `mapstructure:"rule_name,omitempty"`
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
	"Service":         "aws.local.service",
	"Operation":       "aws.local.operation",
	"RemoteService":   "aws.remote.service",
	"RemoteOperation": "aws.remote.operation",
}

func getExactKey(metricDimensionKey string, isTrace bool) string {
	if !isTrace {
		return metricDimensionKey
	}
	traceDimensionKey, ok := traceKeyMap[metricDimensionKey]
	if !ok {
		// return original key if there is no matches
		return metricDimensionKey
	}
	return traceDimensionKey
}

func isSelected(attributes pcommon.Map, selectorMatchers []SelectorMatcherItem, isTrace bool) (bool, error) {
	for _, item := range selectorMatchers {
		exactKey := getExactKey(item.Key, isTrace)
		value, ok := attributes.Get(exactKey)
		if !ok {
			return false, fmt.Errorf("can not find attribute %q in the datapoint", exactKey)
		}
		if !item.Matcher.Match(value.AsString()) {
			return false, nil
		}
	}
	return true, nil
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

func generateTestAttributes(service string, operation string, remoteService string, remoteOperation string,
	isTrace bool) pcommon.Map {
	attributes := pcommon.NewMap()
	if isTrace {
		attributes.PutStr("aws.local.service", service)
		attributes.PutStr("aws.local.operation", operation)
		attributes.PutStr("aws.remote.service", remoteService)
		attributes.PutStr("aws.remote.operation", remoteOperation)
	} else {
		attributes.PutStr("Service", service)
		attributes.PutStr("Operation", operation)
		attributes.PutStr("RemoteService", remoteService)
		attributes.PutStr("RemoteOperation", remoteOperation)
	}
	return attributes
}

func generateActionDetails(rules []Rule, action string) []ActionItem {
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
