// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cumulativetodeltaprocessor

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
)

const (
	// Match types are in internal package from contrib
	// Strict is the FilterType for filtering by exact string matches.
	strict = "strict"
)

type translator struct {
	factory component.ProcessorFactory
}

var _ common.Translator[config.Processor] = (*translator)(nil)

func NewTranslator() common.Translator[config.Processor] {
	return &translator{cumulativetodeltaprocessor.NewFactory()}
}

func (t *translator) Type() config.Type {
	return t.factory.Type()
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config.
// We use cumulative to delta processor with Disk And Net since these metrics are cumulative. We want to know change in value over a time period
func (t *translator) Translate(_ *confmap.Conf) (config.Processor, error) {
	cfg := t.factory.CreateDefaultConfig().(*cumulativetodeltaprocessor.Config)
	cfg.Exclude.MatchType = strict
	cfg.Exclude.Metrics = []string{"iops_in_progress", "diskio_iops_in_progress"}
	return cfg, nil
}
