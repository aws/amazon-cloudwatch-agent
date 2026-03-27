// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetrics

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"

	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/cloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

const systemMetricsNamespace = "CWAgent/System"

type cloudWatchTranslator struct {
	factory exporter.Factory
	isEC2   bool
}

func newCloudWatchTranslator(isEC2 bool) common.ComponentTranslator {
	return &cloudWatchTranslator{factory: cloudwatch.NewFactory(), isEC2: isEC2}
}

func (t *cloudWatchTranslator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), common.PipelineNameSystemMetrics)
}

func (t *cloudWatchTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*cloudwatch.Config)
	credentials := confmap.NewFromStringMap(agent.Global_Config.Credentials)
	_ = credentials.Unmarshal(cfg)

	cfg.Region = agent.Global_Config.Region
	cfg.Namespace = systemMetricsNamespace
	if t.isEC2 {
		cfg.RollupDimensions = [][]string{{"InstanceId"}, {}}
	} else {
		cfg.RollupDimensions = [][]string{{}}
	}
	cfg.MiddlewareID = &agenthealth.MetricsID
	cfg.MaxRetryCount = 2
	cfg.BackoffRetryBase = time.Minute
	cfg.MaxConcurrentPublishers = 1

	return cfg, nil
}
