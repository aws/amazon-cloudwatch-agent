// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscloudwatch

import (
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"

	"github.com/aws/amazon-cloudwatch-agent/internal/metric"
	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/cloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/rollup_dimensions"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

const (
	namespaceKey          = "namespace"
	forceFlushIntervalKey = "force_flush_interval"
	dropOriginalWildcard  = "*"

	internalMaxValuesPerDatum = 5000
)

type translator struct {
	name    string
	factory exporter.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, cloudwatch.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an exporter config based on the fields in the
// metrics section of the JSON config.
// TODO: remove dependency on global config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(common.MetricsKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.MetricsKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*cloudwatch.Config)
	credentials := confmap.NewFromStringMap(agent.Global_Config.Credentials)
	_ = credentials.Unmarshal(cfg)
	cfg.RoleARN = getRoleARN(conf)
	cfg.Region = agent.Global_Config.Region
	if namespace, ok := common.GetString(conf, common.ConfigKey(common.MetricsKey, namespaceKey)); ok {
		cfg.Namespace = namespace
	}
	if endpointOverride, ok := common.GetString(conf, common.ConfigKey(common.MetricsKey, common.EndpointOverrideKey)); ok {
		cfg.EndpointOverride = endpointOverride
	}
	if forceFlushInterval, ok := common.GetDuration(conf, common.ConfigKey(common.MetricsKey, forceFlushIntervalKey)); ok {
		cfg.ForceFlushInterval = forceFlushInterval
	}
	if agent.Global_Config.Internal {
		cfg.MaxValuesPerDatum = internalMaxValuesPerDatum
	}
	if rollupDimensions := getRollupDimensions(conf); rollupDimensions != nil {
		cfg.RollupDimensions = rollupDimensions
	}
	if dropOriginalMetrics := getDropOriginalMetrics(conf); len(dropOriginalMetrics) != 0 {
		cfg.DropOriginalConfigs = dropOriginalMetrics
	}
	cfg.MiddlewareID = &agenthealth.MetricsID
	return cfg, nil
}

func getRoleARN(conf *confmap.Conf) string {
	key := common.ConfigKey(common.MetricsKey, common.CredentialsKey, common.RoleARNKey)
	roleARN, ok := common.GetString(conf, key)
	if !ok {
		roleARN = agent.Global_Config.Role_arn
	}
	return roleARN
}

// TODO: remove dependency on rule.
func getRollupDimensions(conf *confmap.Conf) [][]string {
	key := common.ConfigKey(common.MetricsKey, rollup_dimensions.SectionKey)
	value := conf.Get(key)
	if value == nil {
		return nil
	}
	aggregates, ok := value.([]interface{})
	if !ok || !isValidRollupList(aggregates) {
		return nil
	}
	rollup := make([][]string, len(aggregates))
	for i, aggregate := range aggregates {
		dimensions := aggregate.([]interface{})
		rollup[i] = make([]string, len(dimensions))
		for j, dimension := range dimensions {
			rollup[i][j] = dimension.(string)
		}
	}
	return rollup
}

// isValidRollupList confirms whether the supplied aggregate_dimension is a valid type ([][]string)
func isValidRollupList(aggregates []interface{}) bool {
	if len(aggregates) == 0 {
		return false
	}
	for _, aggregate := range aggregates {
		if dimensions, ok := aggregate.([]interface{}); ok {
			if len(dimensions) != 0 {
				for _, dimension := range dimensions {
					if _, ok := dimension.(string); !ok {
						return false
					}
				}
			}
		} else {
			return false
		}
	}

	return true
}

func getDropOriginalMetrics(conf *confmap.Conf) map[string]bool {
	key := common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey)
	value := conf.Get(key)
	if value == nil {
		return nil
	}
	categories := value.(map[string]interface{})
	dropOriginalMetrics := make(map[string]bool)
	for category := range categories {
		realCategoryName := config.GetRealPluginName(category)
		measurementCfgKey := common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, category, common.MeasurementKey)
		dropOriginalCfgKey := common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, category, common.DropOriginalMetricsKey)
		/* Drop original metrics does not support procstat since procstat can monitor multiple process
		   		"procstat": [
		           {
		             "exe": "W3SVC",
		             "measurement": [
		               "pid_count"
		             ]
		           },
		           {
		             "exe": "IISADMIN",
		             "measurement": [
		               "pid_count"
		             ]
		           }]
		   	Therefore, dropping the original metrics can conflict between these two processes (e.g customers can drop pid_count with the first
		   	process but not the second process)
		*/
		if dropMetrics := common.GetArray[any](conf, dropOriginalCfgKey); dropMetrics != nil {
			for _, dropMetric := range dropMetrics {
				measurements := common.GetArray[any](conf, measurementCfgKey)
				if measurements == nil {
					continue
				}

				dropMetric, ok := dropMetric.(string)
				if !ok {
					continue
				}

				if !strings.Contains(dropMetric, category) && dropMetric != dropOriginalWildcard {
					dropMetric = metric.DecorateMetricName(realCategoryName, dropMetric)
				}
				isMetricDecoration := false
				for _, measurement := range measurements {
					switch val := measurement.(type) {
					/*
						 "disk": {
							"measurement": [
								{
									"name": "free",
									"rename": "DISK_FREE",
									"unit": "unit"
								}
							]
						}
					*/
					case map[string]interface{}:
						metricName, ok := val["name"].(string)
						if !ok {
							continue
						}
						if !strings.Contains(metricName, category) {
							metricName = metric.DecorateMetricName(realCategoryName, metricName)
						}
						// If customers provides drop_original_metrics with a wildcard (*), adding the renamed metric or add the original metric
						// if customers only re-unit the metric
						if strings.Contains(dropMetric, metricName) || dropMetric == dropOriginalWildcard {
							isMetricDecoration = true
							if newMetricName, ok := val["rename"].(string); ok {
								dropOriginalMetrics[newMetricName] = true
							} else {
								dropOriginalMetrics[metricName] = true
							}
						}

					/*
						"measurement": ["free"]
					*/
					case string:
						if dropMetric != dropOriginalWildcard {
							continue
						}
						metricName := val
						if !strings.Contains(metricName, category) {
							metricName = metric.DecorateMetricName(realCategoryName, metricName)
						}

						dropOriginalMetrics[metricName] = true
					default:
						continue
					}
				}

				if !isMetricDecoration && dropMetric != dropOriginalWildcard {
					dropOriginalMetrics[dropMetric] = true
				}

			}
		}
	}
	return dropOriginalMetrics
}
