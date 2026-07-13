// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name             string
	factory          connector.Factory
	errorMode        ottl.ErrorMode
	defaultPipelines []pipeline.ID
	table            []routingconnector.RoutingTableItem
}

type Option func(*translator)

func WithErrorMode(mode ottl.ErrorMode) Option {
	return func(t *translator) {
		t.errorMode = mode
	}
}

func WithDefaultPipelines(pipelines ...pipeline.ID) Option {
	return func(t *translator) {
		t.defaultPipelines = pipelines
	}
}

func WithTable(items ...routingconnector.RoutingTableItem) Option {
	return func(t *translator) {
		t.table = append(t.table, items...)
	}
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(name string, opts ...Option) common.ComponentTranslator {
	t := &translator{
		name:    name,
		factory: routingconnector.NewFactory(),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*routingconnector.Config)
	cfg.ErrorMode = t.errorMode
	cfg.DefaultPipelines = t.defaultPipelines
	cfg.Table = t.table
	return cfg, nil
}
