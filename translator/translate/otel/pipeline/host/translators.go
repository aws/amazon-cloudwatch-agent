// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline"
	adaptertranslator "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/adapter"
)

func NewTranslators(conf *confmap.Conf, os string) (pipeline.TranslatorMap, error) {
	adapterReceivers, err := adaptertranslator.FindReceiversInConfig(conf, os)
	if err != nil {
		return nil, fmt.Errorf("unable to find receivers in config: %w", err)
	}

	translators := common.NewTranslatorMap[*common.ComponentTranslators]()
	// split out delta receiver types
	deltaReceivers := common.NewTranslatorMap[component.Config]()
	hostReceivers := common.NewTranslatorMap[component.Config]()
	adapterReceivers.Range(func(translator common.Translator[component.Config]) {
		if translator.ID().Type() == adapter.Type(common.DiskIOKey) || translator.ID().Type() == adapter.Type(common.NetKey) {
			deltaReceivers.Set(translator)
		} else {
			hostReceivers.Set(translator)
		}
	})

	hasHostPipeline := hostReceivers.Len() != 0
	hasDeltaPipeline := deltaReceivers.Len() != 0

	if hasHostPipeline {
		translators.Set(NewTranslator(common.PipelineNameHost, hostReceivers))
	}
	if hasDeltaPipeline {
		translators.Set(NewTranslator(common.PipelineNameHostDeltaMetrics, deltaReceivers))
	}

	return translators, nil
}
