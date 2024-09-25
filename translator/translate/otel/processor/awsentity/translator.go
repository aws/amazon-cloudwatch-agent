// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const name = "awsentity"

type translator struct {
	factory processor.Factory
}

func NewTranslator() common.Translator[component.Config] {
	return &translator{
		factory: awsentity.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), "")
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*awsentity.Config)

	if common.TelegrafMetricsEnabled(conf) {
		cfg.ScrapeDatapointAttribute = true
	}

	hostedInConfigKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.AppSignals, "hosted_in")
	hostedIn, hostedInConfigured := common.GetString(conf, hostedInConfigKey)
	if !hostedInConfigured {
		hostedInConfigKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.AppSignalsFallback, "hosted_in")
		hostedIn, hostedInConfigured = common.GetString(conf, hostedInConfigKey)
	}
	if common.IsAppSignalsKubernetes() {
		if !hostedInConfigured {
			hostedIn = util.GetClusterNameFromEc2Tagger()
		}
	}

	//TODO: This logic is more or less identical to what AppSignals does. This should be moved to a common place for reuse
	ctx := context.CurrentContext()
	mode := ctx.KubernetesMode()
	cfg.KubernetesMode = mode
	if mode == "" {
		mode = ctx.Mode()
	}
	if mode == config.ModeEC2 {
		if ecsutil.GetECSUtilSingleton().IsECS() {
			mode = config.ModeECS
		}
	}

	switch mode {
	case config.ModeEKS:
		cfg.ClusterName = hostedIn
		cfg.Platform = config.ModeEKS
	case config.ModeK8sEC2:
		cfg.ClusterName = hostedIn
		cfg.Platform = config.ModeK8sEC2
	case config.ModeK8sOnPrem:
		cfg.Platform = config.ModeK8sOnPrem
	case config.ModeEC2:
		cfg.Platform = config.ModeEC2
	case config.ModeECS:
		cfg.Platform = config.ModeECS
	}
	return cfg, nil
}
