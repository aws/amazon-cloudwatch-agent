// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcedetection

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const (
	detectorTypeEC2 = "ec2"
	detectorTypeEKS = "eks"
	detectorTypeEnv = "env"
)

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

func (t *translator) Translate(*confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*resourcedetectionprocessor.Config)
	cfg.Override = true
	cfg.Timeout = time.Second * 2
	c := confmap.NewFromStringMap(map[string]any{
		"ec2": map[string]any{
			"tags": []string{
				"^kubernetes.io/cluster/.*$",
				"^aws:autoscaling:groupName",
			},
		},
	})
	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal into resource detection processor: %w", err)
	}

	mode := context.CurrentContext().KubernetesMode()
	if mode == "" {
		mode = context.CurrentContext().Mode()
	}
	if mode == config.ModeEC2 {
		if ecsutil.GetECSUtilSingleton().IsECS() {
			mode = config.ModeECS
		}
	}

	switch mode {
	case config.ModeEKS:
		cfg.Detectors = []string{detectorTypeEKS, detectorTypeEnv, detectorTypeEC2}
	default:
		cfg.Detectors = []string{detectorTypeEnv, detectorTypeEC2}
	}

	return cfg, nil
}
