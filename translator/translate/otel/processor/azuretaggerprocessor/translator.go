// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azuretaggerprocessor

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/azuretagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// AzuretaggerKey is the config key for Azure append_dimensions
var AzuretaggerKey = common.ConfigKey(common.MetricsKey, common.AppendDimensionsKey)

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslator creates a new azuretagger translator
func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

// NewTranslatorWithName creates a new azuretagger translator with a custom name
func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name, azuretagger.NewFactory()}
}

// ID returns the component ID for this translator
func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config for Azure environments.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(AzuretaggerKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: AzuretaggerKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*azuretagger.Config)

	// Map Azure-specific dimensions from config
	for k, v := range azuretagger.SupportedAppendDimensions {
		value, ok := common.GetString(conf, common.ConfigKey(AzuretaggerKey, k))
		if ok && v == value {
			if k == "VMScaleSetName" {
				// VMScaleSetName comes from tags (like AutoScalingGroupName in AWS)
				cfg.AzureInstanceTagKeys = append(cfg.AzureInstanceTagKeys, k)
			} else {
				// Other dimensions come from IMDS metadata
				cfg.AzureMetadataTags = append(cfg.AzureMetadataTags, k)
			}
		}
	}

	// No refresh by default (tags fetched once at startup)
	cfg.RefreshTagsInterval = 0

	return cfg, nil
}
