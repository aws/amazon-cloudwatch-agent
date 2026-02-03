//go:build !linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name string
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslator creates a new journald receiver translator.
func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

// NewTranslatorWithName creates a new journald receiver translator with a name.
func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.MustNewType("journald"), t.name)
}

// Translate returns an error on non-Linux platforms.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	return nil, errors.New("journald receiver is only supported on Linux")
}
