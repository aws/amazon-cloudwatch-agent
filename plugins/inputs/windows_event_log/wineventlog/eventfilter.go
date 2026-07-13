// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package wineventlog

import (
	"fmt"
	"log"
	"regexp"
)

const (
	includeFilterType = "include"
	excludeFilterType = "exclude"
)

var (
	validFilterTypes    = []string{includeFilterType, excludeFilterType}
	validFilterTypesSet = map[string]struct{}{
		includeFilterType: {},
		excludeFilterType: {},
	}
)

type EventFilter struct {
	Type        string `toml:"type"`
	Expression  string `toml:"expression"`
	expressionP *regexp.Regexp
}

func (filter *EventFilter) init() error {
	if _, ok := validFilterTypesSet[filter.Type]; !ok {
		return fmt.Errorf("filter type %s is incorrect, valid types are: %v", filter.Type, validFilterTypes)
	}

	var err error
	if filter.expressionP, err = regexp.Compile(filter.Expression); err != nil {
		return fmt.Errorf("filter regex has issue, regexp: Compile( %v ): %v", filter.Expression, err.Error())
	}
	return nil
}

func (filter *EventFilter) ShouldPublish(message string) bool {
	if filter.expressionP == nil {
		log.Printf("E! [wineventlog] Filter regex is invalid, expression: %s", filter.Expression)
		return false
	}
	match := filter.expressionP.MatchString(message)
	return (filter.Type == includeFilterType) == match
}
