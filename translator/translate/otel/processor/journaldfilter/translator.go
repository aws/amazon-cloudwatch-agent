// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journaldfilter

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

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

func escapeOTTLString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func (t *filterTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*filterprocessor.Config)

	cfg.ErrorMode = "propagate"

	var logRecordFilters []string
	var includeExprs []string

	for _, filter := range t.filters {
		escaped := escapeOTTLString(filter.Expression)
		switch filter.Type {
		case "exclude":
			logRecordFilters = append(logRecordFilters, fmt.Sprintf(`IsMatch(body, "%s")`, escaped))
		case "include":
			includeExprs = append(includeExprs, fmt.Sprintf(`IsMatch(body, "%s")`, escaped))
		default:
			return nil, fmt.Errorf("unsupported filter type: %s", filter.Type)
		}
	}

	// Combine multiple include filters, drops if none matches.
	if len(includeExprs) == 1 {
		logRecordFilters = append(logRecordFilters, fmt.Sprintf(`not %s`, includeExprs[0]))
	} else if len(includeExprs) > 1 {
		logRecordFilters = append(logRecordFilters, fmt.Sprintf(`not (%s)`, strings.Join(includeExprs, " or ")))
	}

	if len(logRecordFilters) > 0 {
		cfg.Logs = filterprocessor.LogFilters{
			LogConditions: logRecordFilters,
		}
	}

	return cfg, nil
}
