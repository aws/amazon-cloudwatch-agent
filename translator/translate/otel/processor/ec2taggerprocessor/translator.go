// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2taggerprocessor

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/ec2tagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

var Ec2taggerKey = common.ConfigKey(common.MetricsKey, common.AppendDimensionsKey)

type translator struct {
	name    string
	factory processor.Factory
	common.DestinationProvider
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: ec2tagger.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	// Only use named instance for OTLP to avoid changing existing CloudWatch configs
	if t.Destination() == common.OtlpKey {
		t.name = common.OtlpExportKey
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(Ec2taggerKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: Ec2taggerKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*ec2tagger.Config)
	credentials := confmap.NewFromStringMap(agent.Global_Config.Credentials)
	_ = credentials.Unmarshal(cfg)
	for k, v := range ec2tagger.SupportedAppendDimensions {
		value, ok := common.GetString(conf, common.ConfigKey(Ec2taggerKey, k))
		if !ok {
			continue
		}
		if v == value {
			if k == "AutoScalingGroupName" {
				cfg.EC2InstanceTagKeys = append(cfg.EC2InstanceTagKeys, k)
			} else {
				cfg.EC2MetadataTags = append(cfg.EC2MetadataTags, k)
			}
		} else if otelVal, exists := ec2tagger.OTelAppendDimensions[k]; exists && otelVal == value {
			cfg.EC2MetadataTags = append(cfg.EC2MetadataTags, k)
		}
	}

	cfg.RefreshTagsInterval = time.Duration(0)
	cfg.RefreshVolumesInterval = time.Duration(0)

	// Only set middleware when not using OTLP destination (agenthealth not in OTLP pipeline)
	if t.Destination() != common.OtlpKey {
		cfg.MiddlewareID = &agenthealth.StatusCodeID
	}
	cfg.IMDSRetries = retryer.GetDefaultRetryNumber()

	return cfg, nil
}
