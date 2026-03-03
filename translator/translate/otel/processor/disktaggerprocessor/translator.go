// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktaggerprocessor

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata"
	"github.com/aws/amazon-cloudwatch-agent/internal/cloudprovider"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/disktagger"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var diskVolumeKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.DiskKey, common.AppendDimensionsKey)

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return &translator{factory: disktagger.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// IsSet returns true if disk append_dimensions with DiskId is configured.
func IsSet(conf *confmap.Conf) bool {
	if conf == nil {
		return false
	}
	// Check for ${aws:VolumeId} (legacy) or ${disk.id} (OTel) in disk append_dimensions
	value, ok := common.GetString(conf, common.ConfigKey(diskVolumeKey, "VolumeId"))
	if ok && (value == "${aws:VolumeId}" || value == "${disk.id}") {
		return true
	}
	value, ok = common.GetString(conf, common.ConfigKey(diskVolumeKey, "DiskId"))
	return ok && (value == "${aws:VolumeId}" || value == "${disk.id}")
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if !IsSet(conf) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: diskVolumeKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*disktagger.Config)

	// Set cloud provider info at translation time (not runtime)
	if p := cloudmetadata.GetProvider(); p != nil {
		cfg.CloudProvider = p.CloudProvider()
		if p.CloudProvider() == cloudprovider.AWS {
			cfg.InstanceID = p.InstanceID()
			cfg.Region = p.Region()
		}
	}

	return cfg, nil
}
