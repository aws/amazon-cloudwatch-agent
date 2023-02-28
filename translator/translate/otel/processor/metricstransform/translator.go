// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT
package metricstransformprocessor

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

var prometheusKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey)

type translator struct {
	name    string
	factory component.ProcessorFactory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, metricstransformprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(prometheusKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: prometheusKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*metricstransformprocessor.Config)
	var transforms []metricstransformprocessor.Transform
	prometheusTransforms := t.getPrometheusTransforms(conf)
	if prometheusTransforms != nil {
		transforms = append(transforms, prometheusTransforms...)
		cfg.Transforms = transforms
	}
	return cfg, nil
}

func (t *translator) getPrometheusTransforms(conf *confmap.Conf) []metricstransformprocessor.Transform {
	var transforms []metricstransformprocessor.Transform

	ecsSdBaseKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey, "ecs_service_discovery")
	if conf.IsSet(ecsSdBaseKey) {
		// When ECS Service Discovery is enabled, the job name for a target could be specified using 'sd_job_name' in the
		// case of 'task_definition_list' or 'service_name_list_for_tasks'. It could also come from a docker label in the case
		// of 'docker_label' by specifying the label to be used as 'sd_job_name_label'.
		// Once ecs_observer OTel extension figures out the job name using this logic, it writes it as a label in the sd results file.
		// But rather than writing it as 'job' which conflicts with the prometheus-generated 'job', it instead writes it as 'prometheus_job'
		// with the expectation that we rename it back as per our needs, outside the scope of prometheus receiver.
		transforms = append(transforms, metricstransformprocessor.Transform{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:   ".*",
				MatchType: metricstransformprocessor.RegexpMatchType,
			},
			Action: metricstransformprocessor.Update,
			Operations: []metricstransformprocessor.Operation{
				{
					Action:   metricstransformprocessor.UpdateLabel,
					NewLabel: "ServiceName",
					Label:    "prometheus_job", // https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/89a732339795e47bbad4e2d34706fd69f570f5a9/extension/observer/ecsobserver/config.go
				},
			},
		})
	}
	return transforms
}
