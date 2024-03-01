// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsappsignals

import (
	_ "embed"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals"
	appsignalsconfig "github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/config"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/rules"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
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
	cfg := t.factory.CreateDefaultConfig().(*appsignalsconfig.Config)

	hostedInConfigKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.AppSignals, "hosted_in")
	hostedIn, hostedInConfigured := common.GetString(conf, hostedInConfigKey)
	if common.IsAppSignalsKubernetes() {
		if !hostedInConfigured {
			hostedIn = util.GetClusterNameFromEc2Tagger()
		}
	}

	kubernetesMode := context.CurrentContext().KubernetesMode()
	if kubernetesMode == config.ModeEKS {
		cfg.Resolvers = []appsignalsconfig.Resolver{
			appsignalsconfig.NewEKSResolver(hostedIn),
		}
	} else if kubernetesMode == config.ModeK8sEC2 || kubernetesMode == config.ModeK8sOnPrem {
		cfg.Resolvers = []appsignalsconfig.Resolver{
			appsignalsconfig.NewK8sResolver(hostedIn),
		}
	} else {
		cfg.Resolvers = []appsignalsconfig.Resolver{
			appsignalsconfig.NewGenericResolver(hostedIn),
		}
	}

	limiterConfig, _ := t.translateMetricLimiterConfig(conf, configKey)
	cfg.Limiter = limiterConfig

	return t.translateCustomRules(conf, configKey, cfg)
}

func (t *translator) translateMetricLimiterConfig(conf *confmap.Conf, configKey string) (*appsignalsconfig.LimiterConfig, error) {
	limiterConfigKey := common.ConfigKey(configKey, "limiter")
	if !conf.IsSet(limiterConfigKey) {
		return nil, nil
	}

	configJson, ok := conf.Get(limiterConfigKey).(map[string]interface{})
	if !ok {
		return nil, errors.New("type conversion error: limiter is not an object")
	}

	limiterConfig := appsignalsconfig.NewDefaultLimiterConfig()
	if rawVal, exists := configJson["drop_threshold"]; exists {
		if val, ok := rawVal.(float64); !ok {
			return nil, errors.New("type conversion error: drop_threshold is not a number")
		} else {
			limiterConfig.Threshold = int(val)
		}
	}
	if rawVal, exists := configJson["disabled"]; exists {
		if val, ok := rawVal.(bool); !ok {
			return nil, errors.New("type conversion error: disabled is not a boolean")
		} else {
			limiterConfig.Disabled = val
		}
	}
	if rawVal, exists := configJson["log_dropped_metrics"]; exists {
		if val, ok := rawVal.(bool); !ok {
			return nil, errors.New("type conversion error: log_dropped_metrics is not a boolean")
		} else {
			limiterConfig.LogDroppedMetrics = val
		}
	}
	if rawVal, exists := configJson["rotation_interval"]; exists {
		if val, ok := rawVal.(string); !ok {
			return nil, errors.New("type conversion error: rotation_interval is not a string")
		} else {
			if interval, err := time.ParseDuration(val); err != nil {
				return nil, errors.New("type conversion error: rotation_interval is not a time string")
			} else {
				limiterConfig.RotationInterval = interval
			}
		}
	}
	return limiterConfig, nil

}

func (t *translator) translateCustomRules(conf *confmap.Conf, configKey string, cfg *appsignalsconfig.Config) (component.Config, error) {
	var rulesList []rules.Rule
	rulesConfigKey := common.ConfigKey(configKey, common.AppSignalsRules)
	if conf.IsSet(rulesConfigKey) {
		for _, rule := range conf.Get(rulesConfigKey).([]interface{}) {
			ruleConfig := rules.Rule{}
			ruleMap := rule.(map[string]interface{})
			selectors := ruleMap["selectors"].([]interface{})
			action := ruleMap["action"].(string)

			ruleConfig.Selectors = getServiceSelectors(selectors)
			if ruleName, ok := ruleMap["rule_name"]; ok {
				ruleConfig.RuleName = ruleName.(string)
			}

			var err error
			ruleConfig.Action, err = rules.GetAllowListAction(action)
			if err != nil {
				return nil, err
			}
			if ruleConfig.Action == rules.AllowListActionReplace {
				replacements, ok := ruleMap["replacements"]
				if !ok {
					return nil, errors.New("replace action set, but no replacements defined for service rule")
				}
				ruleConfig.Replacements = getServiceReplacements(replacements)
			}

			rulesList = append(rulesList, ruleConfig)
		}
		cfg.Rules = rulesList
	}

	return cfg, nil
}

func getServiceSelectors(selectorsList []interface{}) []rules.Selector {
	var selectors []rules.Selector
	for _, selector := range selectorsList {
		selectorConfig := rules.Selector{}
		selectorsMap := selector.(map[string]interface{})

		selectorConfig.Dimension = selectorsMap["dimension"].(string)
		selectorConfig.Match = selectorsMap["match"].(string)
		selectors = append(selectors, selectorConfig)
	}
	return selectors
}

func getServiceReplacements(replacementsList interface{}) []rules.Replacement {
	var replacements []rules.Replacement
	for _, replacement := range replacementsList.([]interface{}) {
		replacementConfig := rules.Replacement{}
		replacementMap := replacement.(map[string]interface{})

		replacementConfig.TargetDimension = replacementMap["target_dimension"].(string)
		replacementConfig.Value = replacementMap["value"].(string)
		replacements = append(replacements, replacementConfig)
	}
	return replacements
}
