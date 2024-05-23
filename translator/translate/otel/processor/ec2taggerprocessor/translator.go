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
)

const (
	AppendDimensionsKey = "append_dimensions"
)

var ec2taggerKey = common.ConfigKey(common.MetricsKey, AppendDimensionsKey)

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, ec2tagger.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(ec2taggerKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: ec2taggerKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*ec2tagger.Config)
	credentials := confmap.NewFromStringMap(agent.Global_Config.Credentials)
	_ = credentials.Unmarshal(cfg)
	for k, v := range ec2tagger.SupportedAppendDimensions {
		value, ok := common.GetString(conf, common.ConfigKey(ec2taggerKey, k))
		if ok && v == value {
			if k == "AutoScalingGroupName" {
				cfg.EC2InstanceTagKeys = append(cfg.EC2InstanceTagKeys, k)
			} else {
				cfg.EC2MetadataTags = append(cfg.EC2MetadataTags, k)
			}
		}
	}

	if value, ok := common.GetString(conf, common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.DiskKey, AppendDimensionsKey, ec2tagger.AttributeVolumeId)); ok && value == ec2tagger.ValueAppendDimensionVolumeId {
		cfg.EBSDeviceKeys = []string{"*"}
		cfg.DiskDeviceTagKey = "device"
	}

	cfg.RefreshIntervalSeconds = time.Duration(0)
	cfg.IMDSRetries = retryer.GetDefaultRetryNumber()

	return cfg, nil
}
