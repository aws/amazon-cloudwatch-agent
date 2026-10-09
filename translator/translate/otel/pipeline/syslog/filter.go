// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslog

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type filterProcessorTranslator struct {
	name    string
	filters []filter
	factory processor.Factory
}

var _ common.ComponentTranslator = (*filterProcessorTranslator)(nil)

func newFilterProcessorTranslator(name string, filters []filter) common.ComponentTranslator {
	return &filterProcessorTranslator{
		name:    name,
		filters: filters,
		factory: filterprocessor.NewFactory(),
	}
}

func (t *filterProcessorTranslator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *filterProcessorTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	conditions := buildFilterConditions(t.filters)
	if len(conditions) == 0 {
		return t.factory.CreateDefaultConfig(), nil
	}

	cfgMap := map[string]any{
		"error_mode": "ignore",
		"logs": map[string]any{
			"log_record": conditions,
		},
	}

	cfg := t.factory.CreateDefaultConfig()
	if err := confmap.NewFromStringMap(cfgMap).Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal filter processor (%s): %w", t.name, err)
	}
	return cfg, nil
}

// buildFilterConditions converts filters to OTTL drop conditions for the filter processor.
// Include filters keep only matching logs (drop non-matching).
// Exclude filters drop matching logs.
func buildFilterConditions(filters []filter) []string {
	var conditions []string
	for _, f := range filters {
		if f.Expression == "" {
			continue
		}
		switch strings.ToLower(f.Type) {
		case "exclude":
			// Drop logs matching the expression
			conditions = append(conditions, fmt.Sprintf(`IsMatch(body, "%s")`, escapeOTTL(f.Expression)))
		case "include":
			// Drop logs NOT matching the expression
			conditions = append(conditions, fmt.Sprintf(`not IsMatch(body, "%s")`, escapeOTTL(f.Expression)))
		}
	}
	return conditions
}

func escapeOTTL(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
