// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

const (
	TypeStr   = "log_replay"
	stability = component.StabilityLevelAlpha
)

func NewFactory() connector.Factory {
	return connector.NewFactory(
		component.MustNewType(TypeStr),
		createDefaultConfig,
		connector.WithLogsToLogs(createLogsToLogs, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Interval:            defaultInterval,
		SourceFileAttribute: defaultSourceFile,
		MaxStateBytes:       defaultMaxStateBytes,
		MaxRecordBytes:      defaultMaxRecordBytes,
		ErrorMode:           defaultErrorMode,
	}
}

func createLogsToLogs(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
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
