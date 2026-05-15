// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslogrouter

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/processor/syslogrouterprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name string
	cfg  *syslogrouterprocessor.Config
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(name string, cfg *syslogrouterprocessor.Config) common.ComponentTranslator {
	return &translator{
		name: name,
		cfg:  cfg,
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.MustNewType("awssyslogrouter"), t.name)
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	return t.cfg, nil
}
