// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package filestorage

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	directory = "/opt/aws/amazon-cloudwatch-agent/var/file_storage"
)

type translator struct {
	name    string
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// StorageID is the component.ID for the file_storage/journald extension, exported for use by the receiver.
var StorageID component.ID

func NewTranslator() common.ComponentTranslator {
	t := &translator{
		name:    "journald",
		factory: filestorage.NewFactory(),
	}
	StorageID = t.ID()
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*filestorage.Config)
	cfg.Directory = directory
	cfg.CreateDirectory = true
	return cfg, nil
}
