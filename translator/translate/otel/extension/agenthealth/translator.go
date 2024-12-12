// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	translateagent "github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	OperationPutMetricData    = "PutMetricData"
	OperationPutLogEvents     = "PutLogEvents"
	OperationPutTraceSegments = "PutTraceSegments"

	usageDataKey = "usage_data"
)

var (
	MetricsID = component.NewIDWithName(agenthealth.TypeStr, component.DataTypeMetrics.String())
	LogsID    = component.NewIDWithName(agenthealth.TypeStr, component.DataTypeLogs.String())
	TracesID  = component.NewIDWithName(agenthealth.TypeStr, component.DataTypeTraces.String())
)

type translator struct {
	name               string
	operations         []string
	isUsageDataEnabled bool
	factory            extension.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator(name component.DataType, operations []string) common.Translator[component.Config] {
	return &translator{
		name:               name.String(),
		operations:         operations,
		factory:            agenthealth.NewFactory(),
		isUsageDataEnabled: envconfig.IsUsageDataEnabled(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an extension configuration.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*agenthealth.Config)
	cfg.IsUsageDataEnabled = t.isUsageDataEnabled
	if usageData, ok := common.GetBool(conf, common.ConfigKey(common.AgentKey, usageDataKey)); ok {
		cfg.IsUsageDataEnabled = cfg.IsUsageDataEnabled && usageData
	}
	cfg.Stats = agent.StatsConfig{
		Operations: t.operations,
		UsageFlags: map[agent.Flag]any{
			agent.FlagMode:       context.CurrentContext().ShortMode(),
			agent.FlagRegionType: translateagent.Global_Config.RegionType,
		},
	}
	return cfg, nil
}
