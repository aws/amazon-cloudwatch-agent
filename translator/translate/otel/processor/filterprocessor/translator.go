// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package filterprocessor

import (
	_ "embed"
	"fmt"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	matchTypeStrict = "strict"
)

//go:embed filter_jmx_config.yaml
var containerInsightsJmxConfig string

//go:embed filter_containerinsights_config.yaml
var containerInsightsConfig string

//go:embed filter_journald_config.yaml
var journaldConfig string

type translator struct {
	common.NameProvider
	common.IndexProvider
	factory processor.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: filterprocessor.NewFactory()}
	t.SetIndex(-1)
	for _, opt := range opts {
		opt(t)
	}
	if t.Index() != -1 {
		t.SetName(t.Name() + "/" + strconv.Itoa(t.Index()))
	}
	return t
}

var _ common.ComponentTranslator = (*translator)(nil)

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	// Check for journald logs filtering
	journaldKey := common.ConfigKey(common.LogsKey, common.LogsCollectedKey, common.JournaldKey)
	if conf.IsSet(journaldKey) {
		cfg := t.factory.CreateDefaultConfig().(*filterprocessor.Config)
		
		// Generate filter conditions from journald config
		conditions := t.buildJournaldFilters(conf.Get(journaldKey))
		if len(conditions) > 0 {
			c := confmap.NewFromStringMap(map[string]interface{}{
				"error_mode": "ignore",
				"logs": map[string]interface{}{
					"log_record": conditions,
				},
			})
			if err := c.Unmarshal(&cfg); err != nil {
				return nil, fmt.Errorf("unable to unmarshal journald filter processor (%s): %w", t.ID(), err)
			}
			return cfg, nil
		}
		return common.GetYamlFileToYamlConfig(cfg, journaldConfig)
	}

	// also checking for container insights pipeline to add default filtering for prometheus metadata
	if conf == nil || (t.Name() != common.PipelineNameContainerInsights && t.Name() != common.PipelineNameKueue && t.Name() != common.PipelineNameContainerInsightsJmx && !conf.IsSet(common.JmxConfigKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.JmxConfigKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*filterprocessor.Config)
	if t.Name() == common.PipelineNameContainerInsightsJmx {
		return common.GetYamlFileToYamlConfig(cfg, containerInsightsJmxConfig)
	}
	if t.Name() == common.PipelineNameContainerInsights || t.Name() == common.PipelineNameKueue {
		return common.GetYamlFileToYamlConfig(cfg, containerInsightsConfig)
	}

	jmxMap := common.GetIndexedMap(conf, common.JmxConfigKey, t.Index())

	var includeMetricNames []string
	for _, jmxTarget := range common.JmxTargets {
		if targetMap, ok := jmxMap[jmxTarget].(map[string]any); ok {
			includeMetricNames = append(includeMetricNames, common.GetMeasurements(targetMap)...)
		}
	}

	c := confmap.NewFromStringMap(map[string]interface{}{
		"metrics": map[string]any{
			"include": map[string]any{
				"match_type":   matchTypeStrict,
				"metric_names": includeMetricNames,
			},
		},
	})

	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal filter processor (%s): %w", t.ID(), err)
	}

	return cfg, nil
}

// buildJournaldFilters extracts filter conditions from journald config
func (t *translator) buildJournaldFilters(journaldConf interface{}) []string {
	var conditions []string
	journaldMap, ok := journaldConf.(map[string]interface{})
	if !ok {
		return conditions
	}
	collectList, ok := journaldMap["collect_list"].([]interface{})
	if !ok {
		return conditions
	}
	for _, item := range collectList {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		filters, ok := itemMap["filters"].([]interface{})
		if !ok {
			continue
		}
		for _, filter := range filters {
			filterMap, ok := filter.(map[string]interface{})
			if !ok {
				continue
			}
			filterType, _ := filterMap["type"].(string)
			expression, _ := filterMap["expression"].(string)
			if expression == "" {
				continue
			}
			// Exclude filters: drop logs that match the pattern
			if filterType == "exclude" {
				conditions = append(conditions, fmt.Sprintf(`IsMatch(body, "%s")`, expression))
			}
			// Include filters: drop logs that DON'T match the pattern
			if filterType == "include" {
				conditions = append(conditions, fmt.Sprintf(`not IsMatch(body, "%s")`, expression))
			}
		}
	}
	return conditions
}
