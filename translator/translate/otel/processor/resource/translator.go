// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourceprocessor

import (
	"fmt"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
)

type translator struct {
	factory component.ProcessorFactory
}

var _ common.Translator[config.Processor] = (*translator)(nil)

func NewTranslator() common.Translator[config.Processor] {
	return &translator{resourceprocessor.NewFactory()}
}

func (t *translator) Type() config.Type {
	return t.factory.Type()
}

// Translate creates a processor config based on the fields in the
// Metrics section of the JSON config.
func (t *translator) Translate(conf *confmap.Conf) (config.Processor, error) {
	cfg := t.factory.CreateDefaultConfig().(*resourceprocessor.Config)
	var attributes []map[string]interface{}
	prometheusAttributes, err := t.getPrometheusAttributes(conf)
	if err != nil {
		return nil, err
	}
	attributes = append(attributes, prometheusAttributes...)
	c := confmap.NewFromStringMap(map[string]interface{}{
		"attributes": attributes,
	})
	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal resource processor: %w", err)
	}
	return cfg, nil
}

func (t *translator) getPrometheusAttributes(conf *confmap.Conf) ([]map[string]interface{}, error) {
	prometheusKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.PrometheusKey)
	var attributes []map[string]interface{} = nil
	if conf.IsSet(prometheusKey) {
		if !conf.IsSet(common.ConfigKey(prometheusKey, "ecs_service_discovery")) {
			// OTel prometheus receiver stores the job in the service.name label
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/c10532dc4e76de3a703fb485afcffae663dc7b8c/receiver/prometheusreceiver/internal/prom_to_otlp.go#L53
			// We only do this when ECS Service Discovery is not enabled since if it is, the job label
			// would be set using a metrics transform processor originating from 'prometheus_job' label
			// that would have been set by the ecs_observer OTel extension
			attributes = append(attributes, map[string]interface{}{
				"key":            "job",
				"from_attribute": "service.name",
				"action":         "upsert",
			})
		}
		attributes = append(attributes,
			// Delete service.name since we would have already set the job label
			map[string]interface{}{
				"key":    "service.name",
				"action": "delete",
			},
			// OTel prometheus receiver stores the instance in the service.instance.id label
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/c10532dc4e76de3a703fb485afcffae663dc7b8c/receiver/prometheusreceiver/internal/prom_to_otlp.go#L57
			map[string]interface{}{
				"key":            "instance",
				"from_attribute": "service.instance.id",
				"action":         "upsert",
			},
			// Delete service.instance.id since we would have already set the instance label
			map[string]interface{}{
				"key":    "service.instance.id",
				"action": "delete",
			},
			// Delete unnecessary label
			map[string]interface{}{
				"key":    "net.host.port",
				"action": "delete",
			},
			// Delete unnecessary label
			map[string]interface{}{
				"key":    "http.scheme",
				"action": "delete",
			},
			// A meta tag to indicate the AWS EMF Version
			map[string]interface{}{
				"key":    "Version",
				"value":  1,
				"action": "insert",
			},
			// We need to add receiver:prometheus as an attribute to the metrics as awsemfexporter looks for this
			// and if found, adds the 'prom_metric_type' label.
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/c10532dc4e76de3a703fb485afcffae663dc7b8c/exporter/awsemfexporter/metric_translator.go#L149
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/c10532dc4e76de3a703fb485afcffae663dc7b8c/exporter/awsemfexporter/metric_translator.go#L163-L165
			map[string]interface{}{
				"key":    "receiver",
				"value":  "prometheus",
				"action": "insert",
			})
	}
	return attributes, nil
}
