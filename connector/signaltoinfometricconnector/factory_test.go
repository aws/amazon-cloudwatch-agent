// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, component.MustNewType(TypeStr), factory.Type())
	assert.Equal(t, component.StabilityLevelAlpha, factory.LogsToMetricsStability())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, &Config{
		EmitInterval:   defaultEmitInterval,
		EmitOnChange:   true,
		MaxStateBytes:  1048576,
		MaxValueLength: 256,
		ErrorMode:      ottl.IgnoreError,
	}, cfg)
	// The defaults alone are not a usable configuration: rules are required.
	assert.ErrorContains(t, xconfmap.Validate(cfg), "at least one rule must be configured")
}

func TestCreateLogsToMetrics(t *testing.T) {
	factory := NewFactory()
	set := connectortest.NewNopSettings(factory.Type())

	conn, err := factory.CreateLogsToMetrics(context.Background(), set, nil, consumertest.NewNop())
	assert.ErrorContains(t, err, "invalid configuration type")
	assert.Nil(t, conn)

	cfg := validConfig()
	require.NoError(t, cfg.Validate())
	conn, err = factory.CreateLogsToMetrics(context.Background(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, conn)

	assert.Equal(t, consumer.Capabilities{MutatesData: false}, conn.Capabilities())
	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, conn.ConsumeLogs(context.Background(), plog.NewLogs()))
	assert.NoError(t, conn.Shutdown(context.Background()))
}
