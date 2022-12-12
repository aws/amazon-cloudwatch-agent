// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extension

import (
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"log"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

type translator struct {
	translators []common.Translator[config.Extension]
}

var _ common.Translator[common.Extensions] = (*translator)(nil)

func NewTranslator(translators ...common.Translator[config.Extension]) common.Translator[common.Extensions] {
	return &translator{translators}
}

// Type is unused.
func (t *translator) Type() config.Type {
	return ""
}

// Translate creates the extension configuration.
func (t *translator) Translate(conf *confmap.Conf) (common.Extensions, error) {
	extensions := make(common.Extensions)
	for _, et := range t.translators {
		if extension, err := et.Translate(conf); extension != nil {
			extensions[extension.ID()] = extension
		} else if err != nil {
			log.Printf("W! ignoring translation of ecs_observer due to: %v", err)
		}
	}
	return extensions, nil
}
