// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	otlpBasePathKey         = common.OTLPLogsKey
	otlpEMFProcessorPathKey = common.ConfigKey(otlpBasePathKey, common.EMFProcessorKey)
)

// setOTLPFields configures the EMF exporter for OTLP metrics
func setOTLPFields(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	// Set log group name if provided
	setOTLPLogGroup(conf, cfg)

	// Set EMF processor fields if provided
	if conf.IsSet(otlpEMFProcessorPathKey) {
		if err := setOTLPNamespace(conf, cfg); err != nil {
			return err
		}
		if err := setOTLPMetricDescriptors(conf, cfg); err != nil {
			return err
		}
	}

	return nil
}

// setOTLPLogGroup sets the log group name for OTLP metrics
// If not set, use the default from awsemf_default_generic.yaml (/aws/cwagent)
func setOTLPLogGroup(conf *confmap.Conf, cfg *awsemfexporter.Config) {
	if logGroupName, ok := common.GetString(conf, common.ConfigKey(otlpBasePathKey, common.LogGroupName)); ok {
		cfg.LogGroupName = logGroupName
	}
}

// setOTLPNamespace sets the metric namespace for OTLP metrics
func setOTLPNamespace(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	return setNamespaceWithDefault(conf, common.ConfigKey(otlpEMFProcessorPathKey, metricNamespace), "", cfg)
}

// setOTLPMetricDescriptors sets the metric units for OTLP metrics
func setOTLPMetricDescriptors(conf *confmap.Conf, cfg *awsemfexporter.Config) error {
	return setMetricDescriptors(conf, common.ConfigKey(otlpEMFProcessorPathKey, metricUnit), cfg)
}
