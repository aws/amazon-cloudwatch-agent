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

	cfg.KubernetesMode = context.CurrentContext().KubernetesMode()
	switch cfg.KubernetesMode {
	case config.ModeEKS:
		cfg.ClusterName = hostedIn
	}
	return cfg, nil
}
