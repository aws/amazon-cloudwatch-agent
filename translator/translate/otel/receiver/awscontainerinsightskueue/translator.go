// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscontainerinsightskueue

import (
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	defaultMetricsCollectionInterval = -1 // default to -1 to use default value defined in receiver
)

type translator struct {
	name    string
	factory receiver.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

// NewTranslator creates a new aws container insight receiver translator.
func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{
		name:    name,
		factory: awscontainerinsightskueuereceiver.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an aws container insights kueue receiver config if either
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
	cfg.ClusterName = common.GetClusterName(conf)

	if cfg.ClusterName == "" {
		return errors.New("cluster name is not provided and was not auto-detected from EC2 tags")
	}
	return nil
}
