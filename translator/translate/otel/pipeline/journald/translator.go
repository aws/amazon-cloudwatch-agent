// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"fmt"
	"strconv"

	journaldinput "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/journald"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/filestorage"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/journaldfilter"
	journaldreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/journald"
)

type translator struct {
	name string
	common.IndexProvider
}

var _ common.PipelineTranslator = (*translator)(nil)

const (
	pipelineName = "journald"
)

func NewTranslator(opts ...common.TranslatorOption) common.PipelineTranslator {
	t := &translator{name: pipelineName}
	t.SetIndex(-1)
	for _, opt := range opts {
		opt(t)
	}
	if t.Index() != -1 {
		t.name += "/" + strconv.Itoa(t.Index())
	}
	return t
}

func NewTranslators(conf *confmap.Conf) common.TranslatorMap[*common.ComponentTranslators, pipeline.ID] {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()

	journaldKey := common.ConfigKey(common.LogsKey, "logs_collected", "journald")
	if conf == nil || !conf.IsSet(journaldKey) {
		return translators
	}

	journaldConf, err := conf.Sub(journaldKey)
	if err != nil || journaldConf == nil {
		return translators
	}

	if collectList, ok := journaldConf.Get("collect_list").([]any); ok {
		for index := range collectList {
			translators.Set(NewTranslator(common.WithIndex(index)))
		}
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

	// Get the specific collect_list entry for this pipeline
	index := t.Index()
	if index < 0 || index >= len(collectListSlice) {
		return nil, fmt.Errorf("invalid collect_list index %d", index)
	}

	entryConfig, ok := collectListSlice[index].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid collect_list entry at index %d", index)
	}

	translators := common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}

	// Create a unique name suffix based on index
	suffix := fmt.Sprintf("_%d", index)

	// Add journald receiver for this entry
	receiverName := "journald" + suffix
	units, _ := entryConfig["units"].([]interface{})
	var unitStrings []string
	for _, unit := range units {
		if unitStr, ok := unit.(string); ok {
			unitStrings = append(unitStrings, unitStr)
		}
	}

	// Extract priority
	priority, _ := entryConfig["priority"].(string)

	// Extract matches
	var matchConfigs []journaldinput.MatchConfig
	if matches, ok := entryConfig["matches"].([]interface{}); ok {
		for _, match := range matches {
			if matchMap, ok := match.(map[string]interface{}); ok {
				mc := make(journaldinput.MatchConfig)
				for k, v := range matchMap {
					if vs, ok := v.(string); ok {
						mc[k] = vs
					}
				}
				if len(mc) > 0 {
					matchConfigs = append(matchConfigs, mc)
				}
			}
		}
	}

	translators.Receivers.Set(journaldreceiver.NewTranslatorWithConfig(receiverName, unitStrings, priority, matchConfigs))

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
	translators.Exporters.Set(journald.NewTranslatorWithConfig(exporterName, entryConfig))

	// Add health extension
	translators.Extensions.Set(agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}))
	translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true))

	// Add file storage extension for journald cursor persistence
	translators.Extensions.Set(filestorage.NewTranslator())

	return &translators, nil
}
