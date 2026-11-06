// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

const (
	// TypeStr is the component type of the connector.
	TypeStr   = "signal_to_info_metric"
	stability = component.StabilityLevelAlpha
)

// NewFactory returns the factory for the signal_to_info_metric connector,
// which consumes logs and emits metrics.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		component.MustNewType(TypeStr),
		createDefaultConfig,
		connector.WithLogsToMetrics(createLogsToMetrics, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		EmitInterval:   defaultEmitInterval,
		EmitOnChange:   defaultEmitOnChange,
		MaxStateBytes:  defaultMaxStateBytes,
		MaxValueLength: defaultMaxValueLength,
		ErrorMode:      defaultErrorMode,
	}
}

func createLogsToMetrics(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Logs, error) {
	c, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid configuration type: %T", cfg)
	}
	conn, err := newConnector(set, c, nextConsumer)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
