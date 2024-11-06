// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscontainerinsightskueue

import (
	"errors"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	defaultMetricsCollectionInterval = time.Minute
)

type translator struct {
	name    string
	factory receiver.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

// NewTranslator creates a new aws container insight receiver translator.
func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{
		name:    name,
		factory: awscontainerinsightskueuereceiver.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an aws container insights receiver config if either
// of the sections defined in the services exist.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*awscontainerinsightskueuereceiver.Config)
	intervalKeyChain := []string{
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	cfg.CollectionInterval = common.GetOrDefaultDuration(conf, intervalKeyChain, defaultMetricsCollectionInterval)

	if err := t.setClusterName(conf, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (t *translator) setClusterName(conf *confmap.Conf, cfg *awscontainerinsightskueuereceiver.Config) error {
	clusterNameKey := common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, "cluster_name")
	if clusterName, ok := common.GetString(conf, clusterNameKey); ok {
		cfg.ClusterName = clusterName
	} else {
		cfg.ClusterName = util.GetClusterNameFromEc2Tagger()
	}

	if cfg.ClusterName == "" {
		return errors.New("cluster name is not provided and was not auto-detected from EC2 tags")
	}
	return nil
}

func KueueContainerInsightsEnabled(conf *confmap.Conf) bool {
	return common.GetOrDefaultBool(conf, common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnableKueueContainerInsights), false)
}
