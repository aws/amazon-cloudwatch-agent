// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsappsignals

import (
	_ "embed"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/customconfiguration"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name     string
	dataType component.DataType
	factory  processor.Factory
}

type Option interface {
	apply(t *translator)
}

type optionFunc func(t *translator)

func (o optionFunc) apply(t *translator) {
	o(t)
}

// WithDataType determines where the translator should look to find
// the configuration.
func WithDataType(dataType component.DataType) Option {
	return optionFunc(func(t *translator) {
		t.dataType = dataType
	})
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[component.Config] {
	t := &translator{factory: awsappsignals.NewFactory()}
	for _, opt := range opts {
		opt.apply(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	configKey := common.AppSignalsConfigKeys[t.dataType]
	cfg := t.factory.CreateDefaultConfig().(*awsappsignals.Config)
	if common.IsAppSignalsKubernetes() {
		cfg.Resolvers = []string{"eks"}
	} else {
		cfg.Resolvers = []string{"generic"}
	}
	return t.translateCustomRules(conf, configKey, cfg)
}

func (t *translator) translateCustomRules(conf *confmap.Conf, configKey string, cfg *awsappsignals.Config) (component.Config, error) {
	var rules []customconfiguration.Rule
	rulesConfigKey := common.ConfigKey(configKey, common.AppSignalsRules)
	if conf.IsSet(rulesConfigKey) {
		for _, rule := range conf.Get(rulesConfigKey).([]interface{}) {
			ruleConfig := customconfiguration.Rule{}
			ruleMap := rule.(map[string]interface{})
			selectors := ruleMap["selectors"].([]interface{})
			action := ruleMap["action"].(string)

			ruleConfig.Selectors = getServiceSelectors(selectors)
			if ruleName, ok := ruleMap["rule_name"]; ok {
				ruleConfig.RuleName = ruleName.(string)
			}

			var err error
			ruleConfig.Action, err = customconfiguration.GetAllowListAction(action)
			if err != nil {
				return nil, err
			}
			if ruleConfig.Action == customconfiguration.AllowListActionReplace {
				replacements, ok := ruleMap["replacements"]
				if !ok {
					return nil, errors.New("replace action set, but no replacements defined for service rule")
				}
				ruleConfig.Replacements = getServiceReplacements(replacements)
			}

			rules = append(rules, ruleConfig)
		}
		cfg.Rules = rules
	}

	return cfg, nil
}

func getServiceSelectors(selectorsList []interface{}) []customconfiguration.Selector {
	var selectors []customconfiguration.Selector
	for _, selector := range selectorsList {
		selectorConfig := customconfiguration.Selector{}
		selectorsMap := selector.(map[string]interface{})

		selectorConfig.Dimension = selectorsMap["dimension"].(string)
		selectorConfig.Match = selectorsMap["match"].(string)
		selectors = append(selectors, selectorConfig)
	}
	return selectors
}

func getServiceReplacements(replacementsList interface{}) []customconfiguration.Replacement {
	var replacements []customconfiguration.Replacement
	for _, replacement := range replacementsList.([]interface{}) {
		replacementConfig := customconfiguration.Replacement{}
		replacementMap := replacement.(map[string]interface{})

		replacementConfig.TargetDimension = replacementMap["target_dimension"].(string)
		replacementConfig.Value = replacementMap["value"].(string)
		replacements = append(replacements, replacementConfig)
	}
	return replacements
}
