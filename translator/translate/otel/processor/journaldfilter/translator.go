// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journaldfilter

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{
		name:    name,
		factory: filterprocessor.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*filterprocessor.Config)
	
	// Set error mode to ignore to prevent pipeline failures
	cfg.ErrorMode = "ignore"
	
	return cfg, nil
}

// NewTranslatorWithFilters creates a filter processor with specific OTTL expressions
func NewTranslatorWithFilters(name string, filters []FilterConfig) common.ComponentTranslator {
	return &filterTranslator{
		name:    name,
		factory: filterprocessor.NewFactory(),
		filters: filters,
	}
}

type FilterConfig struct {
	Type       string `json:"type"`       // "include" or "exclude"
	Expression string `json:"expression"` // regex pattern
}

type filterTranslator struct {
	name    string
	factory processor.Factory
	filters []FilterConfig
}

var _ common.ComponentTranslator = (*filterTranslator)(nil)

func (t *filterTranslator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *filterTranslator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*filterprocessor.Config)
	
	// Set error mode to ignore to prevent pipeline failures
	cfg.ErrorMode = "ignore"
	
	// Convert filters to OTTL expressions
	var logRecordFilters []string
	for _, filter := range t.filters {
		var ottlExpr string
		switch filter.Type {
		case "exclude":
			// For exclude filters, we want to drop logs that match the pattern
			// Filter processor drops when condition is TRUE, so use IsMatch directly
			ottlExpr = fmt.Sprintf(`IsMatch(body, "%s")`, filter.Expression)
		case "include":
			// For include filters, we want to keep only logs that match the pattern
			// Filter processor drops when condition is TRUE, so use NOT IsMatch to drop non-matching logs
			ottlExpr = fmt.Sprintf(`not IsMatch(body, "%s")`, filter.Expression)
		default:
			return nil, fmt.Errorf("unsupported filter type: %s", filter.Type)
		}
		logRecordFilters = append(logRecordFilters, ottlExpr)
	}
	
	if len(logRecordFilters) > 0 {
		cfg.Logs = filterprocessor.LogFilters{
			LogConditions: logRecordFilters,
		}
	}
	
	return cfg, nil
}