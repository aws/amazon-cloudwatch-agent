// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcedetection

import (
	_ "embed"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

//go:embed configs/config.yaml
var appSignalsDefaultResourceDetectionConfig string

//go:embed configs/ecs_config.yaml
var appSignalsECSResourceDetectionConfig string

type translator struct {
	name     string
	dataType component.DataType
	factory  processor.Factory
}

type Option interface {
	apply(t *translator)
}

type optionFunc func(t *translator)

func (o optionFunc) apply(t *translator) {
	o(t)
}

// WithDataType determines where the translator should look to find
// the configuration.
func WithDataType(dataType component.DataType) Option {
	return optionFunc(func(t *translator) {
		t.dataType = dataType
	})
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[component.Config] {
	t := &translator{factory: resourcedetectionprocessor.NewFactory()}
	for _, opt := range opts {
		opt.apply(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*resourcedetectionprocessor.Config)
	cfg.MiddlewareID = &agenthealth.StatusCodeID
	mode := context.CurrentContext().KubernetesMode()
	if mode == "" {
		mode = context.CurrentContext().Mode()
	}
	if mode == config.ModeEC2 {
		if ecsutil.GetECSUtilSingleton().IsECS() {
			mode = config.ModeECS
		}
	}
	cfg.MiddlewareID = &agenthealth.StatusCodeID
	switch mode {
	case config.ModeECS:
		return common.GetYamlFileToYamlConfig(cfg, appSignalsECSResourceDetectionConfig)
	default:
		return common.GetYamlFileToYamlConfig(cfg, appSignalsDefaultResourceDetectionConfig)
	}

}
