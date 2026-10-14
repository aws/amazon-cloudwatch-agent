// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nodemetadataenricher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

// dummyConfig is an alternate component.Config used to exercise the
// configuration-type assertion error path in createMetricsProcessor and
// createLogsProcessor. It does NOT embed the processor's *Config type, so the
// `cfg.(*Config)` assertion in the factory functions should fail.
type dummyConfig struct{}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()

	require.NotNil(t, factory)
	assert.Equal(t, TypeStr, factory.Type())

	// Both metric and log signal stability levels should be set.
	assert.Equal(t, stability, factory.MetricsStability())
	assert.Equal(t, stability, factory.LogsStability())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()

	require.NotNil(t, cfg)
	_, ok := cfg.(*Config)
	assert.True(t, ok, "createDefaultConfig should return *Config")
}

func TestCreateMetricsProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	p, err := factory.CreateMetrics(
		context.Background(),
		processortest.NewNopSettings(factory.Type()),
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.True(t, p.Capabilities().MutatesData, "processor should declare it mutates data")
}

func TestCreateMetricsProcessor_BadConfigType(t *testing.T) {
	p, err := createMetricsProcessor(
		context.Background(),
		processortest.NewNopSettings(component.MustNewType("nodemetadataenricher")),
		&dummyConfig{},
		consumertest.NewNop(),
	)
	assert.Error(t, err, "createMetricsProcessor should reject a config of the wrong type")
	assert.Nil(t, p)
}

func TestCreateLogsProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	p, err := factory.CreateLogs(
		context.Background(),
		processortest.NewNopSettings(factory.Type()),
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.True(t, p.Capabilities().MutatesData, "processor should declare it mutates data")
}

func TestCreateLogsProcessor_BadConfigType(t *testing.T) {
	p, err := createLogsProcessor(
		context.Background(),
		processortest.NewNopSettings(component.MustNewType("nodemetadataenricher")),
		&dummyConfig{},
		consumertest.NewNop(),
	)
	assert.Error(t, err, "createLogsProcessor should reject a config of the wrong type")
	assert.Nil(t, p)
}
