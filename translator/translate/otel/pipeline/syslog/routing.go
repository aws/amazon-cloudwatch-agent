// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslog

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type routingConnectorTranslator struct {
	name             string
	factory          connector.Factory
	defaultPipelines []pipeline.ID
	table            []routingTableEntry
}

type routingTableEntry struct {
	condition string
	pipelines []pipeline.ID
}

var _ common.ComponentTranslator = (*routingConnectorTranslator)(nil)

func newRoutingConnectorTranslator(name string, defaultPipelines []pipeline.ID, table []routingTableEntry) common.ComponentTranslator {
	return &routingConnectorTranslator{
		name:             name,
		factory:          routingconnector.NewFactory(),
		defaultPipelines: defaultPipelines,
		table:            table,
	}
}

func (t *routingConnectorTranslator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *routingConnectorTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	var tableItems []map[string]any
	for _, entry := range t.table {
		tableItems = append(tableItems, map[string]any{
			"context":   "log",
			"condition": entry.condition,
			"pipelines": pipelineIDsToStrings(entry.pipelines),
		})
	}

	cfgMap := map[string]any{
		"default_pipelines": pipelineIDsToStrings(t.defaultPipelines),
		"error_mode":        "ignore",
		"table":             tableItems,
	}

	cfg := t.factory.CreateDefaultConfig()
	if err := confmap.NewFromStringMap(cfgMap).Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func pipelineIDsToStrings(ids []pipeline.ID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
