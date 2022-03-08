// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logfile

import (
	"fmt"
	"regexp"

	"github.com/aws/amazon-cloudwatch-agent/logs"
)

const (
	includeFilterType = "include"
	excludeFilterType = "exclude"
)

var (
	validFilterTypes    = []string{includeFilterType, excludeFilterType}
	validFilterTypesSet = map[string]bool{
		includeFilterType: true,
		excludeFilterType: true,
	}
)

type LogFilter struct {
	Type        string `toml:"type"`
	Expression  string `toml:"expression"`
	expressionP *regexp.Regexp
}

func (filter *LogFilter) init() error {
	if _, present := validFilterTypesSet[filter.Type]; !present {
		return fmt.Errorf("filter type %s is incorrect, valid types are: %v", filter.Type, validFilterTypes)
	}

	var err error
	if filter.expressionP, err = regexp.Compile(filter.Expression); err != nil {
		return fmt.Errorf("filter regex has issue, regexp: Compile( %v ): %v", filter.Expression, err.Error())
	}
	return nil
}

func (filter *LogFilter) ShouldPublish(event logs.LogEvent) bool {
	match := filter.expressionP.MatchString(event.Message())
	return (filter.Type == includeFilterType) == match
}
