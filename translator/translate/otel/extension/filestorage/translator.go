// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package filestorage

import (
	"os"
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// ComponentID returns the component.ID for a named file_storage extension.
func ComponentID(name string) component.ID {
	return component.NewIDWithName(filestorage.NewFactory().Type(), name)
}

func NewTranslator(name string) common.ComponentTranslator {
	return &translator{
		name:    name,
		factory: filestorage.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*filestorage.Config)
	cfg.Directory = filepath.Join(util.GetFileStateFolder(), "otel")
	cfg.Compaction.Directory = os.TempDir()
	cfg.CreateDirectory = true
	return cfg, nil
}
