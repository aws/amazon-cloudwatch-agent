// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extension

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

type translator struct {
	translators []common.Translator[component.Config]
}

var _ common.Translator[common.Extensions] = (*translator)(nil)

func NewTranslator(translators ...common.Translator[component.Config]) common.Translator[common.Extensions] {
	return &translator{translators}
}

// Type is unused.
func (t *translator) Type() component.Type {
	return ""
}

// Translate creates the extension configuration.
func (t *translator) Translate(conf *confmap.Conf, translatorOptions common.TranslatorOptions) (common.Extensions, error) {
	extensions := make(common.Extensions)
	for _, et := range t.translators {
		if extension, err := et.Translate(conf, translatorOptions); extension != nil {
			extensions[extension.ID()] = extension
		} else if err != nil {
			log.Printf("W! ignoring translation of %v due to: %v", et.Type(), err)
		}
	}
	return extensions, nil
}
