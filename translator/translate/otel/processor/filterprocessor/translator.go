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
	// also checking for container insights pipeline to add default filtering for prometheus metadata
	if conf == nil || (t.Name() != common.PipelineNameContainerInsights && t.Name() != common.PipelineNameContainerInsightsJmx && !conf.IsSet(common.JmxConfigKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.JmxConfigKey}
	}

	cfg := t.factory.CreateDefaultConfig().(*filterprocessor.Config)
	if t.Name() == common.PipelineNameContainerInsightsJmx {
		return common.GetYamlFileToYamlConfig(cfg, containerInsightsJmxConfig)
	}
	if t.Name() == common.PipelineNameContainerInsights {
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
