// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metricstarttime

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	factory processor.Factory
	common.NameProvider
}

var _ common.ComponentTranslator = (*translator)(nil)
var _ common.NameSetter = (*translator)(nil)

// NewTranslator returns a translator for the metricstarttime processor. The
// processor's default strategy (true_reset_point) stamps StartTimeUnixNano on
// sums and histograms, which CloudWatch's OTLP ingestion requires.
func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: metricstarttimeprocessor.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return NewTranslator(common.WithName(name))
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

// Translate returns the processor's default config (true_reset_point strategy).
func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	return t.factory.CreateDefaultConfig(), nil
}
