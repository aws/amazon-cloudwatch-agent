// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslogrouterprocessor

import (
	"fmt"
	"regexp"
)

type Filter struct {
	Type       string `mapstructure:"type"`
	Expression string `mapstructure:"expression"`
}

type Config struct {
	Rule            MatchRule   `mapstructure:"rule"`
	PriorRules      []MatchRule `mapstructure:"prior_rules"`
	IsDefault       bool        `mapstructure:"is_default"`
	AllRules        []MatchRule `mapstructure:"all_rules"`
	ListenerFilters []Filter    `mapstructure:"listener_filters"`
	RuleFilters     []Filter    `mapstructure:"rule_filters"`
}

type MatchRule struct {
	Hostname string `mapstructure:"hostname"`
	Facility *int   `mapstructure:"facility"`
	AppName  string `mapstructure:"app_name"`
}

func (c *Config) Validate() error {
	if !c.IsDefault {
		if c.Rule.Hostname == "" && c.Rule.Facility == nil && c.Rule.AppName == "" {
			return fmt.Errorf("rule must have at least one non-empty field when is_default is false")
		}
	}
	rules := append([]MatchRule{c.Rule}, c.PriorRules...)
	rules = append(rules, c.AllRules...)
	for _, r := range rules {
		if r.Facility != nil && (*r.Facility < 0 || *r.Facility > 23) {
			return fmt.Errorf("facility must be between 0 and 23, got %d", *r.Facility)
		}
	}
	for _, f := range append(c.ListenerFilters, c.RuleFilters...) {
		if f.Type != "include" && f.Type != "exclude" {
			return fmt.Errorf("filter type must be 'include' or 'exclude', got %q", f.Type)
		}
		if _, err := regexp.Compile(f.Expression); err != nil {
			return fmt.Errorf("invalid filter expression %q: %v", f.Expression, err)
		}
	}
	return nil
}
