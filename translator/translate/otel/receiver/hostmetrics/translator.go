// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	baseKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.LoadKey)
)

const (
	defaultCollectionInterval = time.Minute
)

type translator struct {
	common.NameProvider
	factory receiver.Factory
}

func NewTranslator(
	opts ...common.TranslatorOption,
) common.ComponentTranslator {
	t := &translator{factory: hostmetricsreceiver.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.Name())
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(baseKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: baseKey}
	}

	intervalKeyChain := []string{
		common.ConfigKey(baseKey, common.MetricsCollectionIntervalKey),
		common.ConfigKey(common.AgentKey, common.MetricsCollectionIntervalKey),
	}
	interval := common.GetOrDefaultDuration(conf, intervalKeyChain, defaultCollectionInterval)

	return map[string]interface{}{
		"collection_interval": interval.String(),
		"scrapers": map[string]interface{}{
			common.LoadKey: struct{}{},
		},
	}, nil
}