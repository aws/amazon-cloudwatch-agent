// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package filestorage

import (
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func ComponentID() component.ID {
	return component.NewIDWithName(filestorage.NewFactory().Type(), common.OpenTelemetryKey)
}

func NewTranslator() common.ComponentTranslator {
	return &translator{
		factory: filestorage.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), common.OpenTelemetryKey)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*filestorage.Config)
	dir := filepath.Join(util.GetFileStateFolder(), "otel")
	cfg.Directory = dir
	cfg.Compaction.Directory = dir
	cfg.CreateDirectory = true
	return cfg, nil
}
