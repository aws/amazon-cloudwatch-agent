// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsattributelimit

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsattributelimitprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// maxTotalAttributes is the CloudWatch OTLP backend hard limit. The endpoint
// rejects the whole datapoint (HTTP 400) when resource+scope+datapoint
// attributes exceed this. The shared metrics pipeline caps here after
// enrichment as a pure safety net: it is a no-op under 150 and, when over,
// the processor's built-in tier logic sheds low-value labels first while
// protecting identity/cloud/host/datapoint attributes.
const maxTotalAttributes = 150

type translator struct {
	factory processor.Factory
	common.NameProvider
}

var _ common.ComponentTranslator = (*translator)(nil)
var _ common.NameSetter = (*translator)(nil)

// NewTranslator creates a new awsattributelimit processor translator with options.
func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: awsattributelimitprocessor.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*awsattributelimitprocessor.Config)
	cfg.MaxTotalAttributes = maxTotalAttributes
	return cfg, nil
}
