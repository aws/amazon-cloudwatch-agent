// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package filestorage

import (
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	name = "journald"
)

type translator struct {
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// StorageComponentID returns the component.ID for the file_storage/journald extension.
func StorageComponentID() component.ID {
	return component.NewIDWithName(filestorage.NewFactory().Type(), name)
}

func NewTranslator() common.ComponentTranslator {
	return &translator{
		factory: filestorage.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*filestorage.Config)
	cfg.Directory = filepath.Join(paths.AgentDir, "logs", "state")
	cfg.CreateDirectory = true
	return cfg, nil
}
