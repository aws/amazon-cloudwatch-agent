// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/journald"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/journaldfilter"
	journaldreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/journald"
)

type translator struct {
	name string
}

var _ common.PipelineTranslator = (*translator)(nil)

const (
	pipelineName = "journald"
)

func NewTranslator() common.PipelineTranslator {
	return &translator{name: pipelineName}
}

func NewTranslators(conf *confmap.Conf) common.TranslatorMap[*common.ComponentTranslators, pipeline.ID] {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	
	journaldKey := common.ConfigKey(common.LogsKey, "logs_collected", "journald")
	if conf != nil && conf.IsSet(journaldKey) {
		// Create a single pipeline translator that handles all collect_list entries
		translators.Set(NewTranslator())
	}
	
	return translators
}



func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalLogs, t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	journaldKey := common.ConfigKey(common.LogsKey, "logs_collected", "journald")
	if conf == nil || !conf.IsSet(journaldKey) {
		return nil, &common.MissingKeyError{ID: component.NewID(component.MustNewType("journald")), JsonKey: journaldKey}
	}

	// Get the journald configuration
	journaldConf, err := conf.Sub(journaldKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get journald configuration: %w", err)
	}
	if journaldConf == nil {
		return nil, fmt.Errorf("journald configuration not found")
	}

	collectList := journaldConf.Get("collect_list")
	if collectList == nil {
		return nil, fmt.Errorf("collect_list not found in journald configuration")
	}

	collectListSlice, ok := collectList.([]interface{})
	if !ok || len(collectListSlice) == 0 {
		return nil, fmt.Errorf("collect_list is empty or invalid")
	}

	translators := common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}

	// Process each collect_list entry to create separate components
	for i, collectEntry := range collectListSlice {
		entryConfig, ok := collectEntry.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid collect_list entry at index %d", i)
		}

		// Create unique suffix for multiple entries
		suffix := ""
		if len(collectListSlice) > 1 {
			suffix = fmt.Sprintf("_%d", i)
		}

		// Add journald receiver for this entry
		receiverName := "journald" + suffix
		units, _ := entryConfig["units"].([]interface{})
		var unitStrings []string
		for _, unit := range units {
			if unitStr, ok := unit.(string); ok {
				unitStrings = append(unitStrings, unitStr)
			}
		}
		translators.Receivers.Set(journaldreceiver.NewTranslatorWithUnits(receiverName, unitStrings))

		// Add filter processor if filters are specified
		if filters, ok := entryConfig["filters"].([]interface{}); ok && len(filters) > 0 {
			var filterConfigs []journaldfilter.FilterConfig
			for _, filter := range filters {
				if filterMap, ok := filter.(map[string]interface{}); ok {
					filterType, _ := filterMap["type"].(string)
					expression, _ := filterMap["expression"].(string)
					if filterType != "" && expression != "" {
						filterConfigs = append(filterConfigs, journaldfilter.FilterConfig{
							Type:       filterType,
							Expression: expression,
						})
					}
				}
			}
			if len(filterConfigs) > 0 {
				filterName := "journald" + suffix
				translators.Processors.Set(journaldfilter.NewTranslatorWithFilters(filterName, filterConfigs))
			}
		}

		// Add batch processor for performance
		batchName := "journald" + suffix
		translators.Processors.Set(batchprocessor.NewTranslatorWithNameAndSection(batchName, common.LogsKey))

		// Add journald exporter with specific config for this collect_list entry
		exporterName := "journald" + suffix
		journaldExporter := journald.NewTranslatorWithConfig(exporterName, entryConfig)
		translators.Exporters.Set(journaldExporter)
	}

	// Add health extension
	translators.Extensions.Set(agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}))
	translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true))

	return &translators, nil
}

