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
	Type       string `json:"type"`       
	Expression string `json:"expression"` 
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

	cfg.ErrorMode = "ignore"

	var logRecordFilters []string
	for _, filter := range t.filters {
		var ottlExpr string
		switch filter.Type {
		case "exclude":
			ottlExpr = fmt.Sprintf(`IsMatch(body, "%s")`, filter.Expression)
		case "include":
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