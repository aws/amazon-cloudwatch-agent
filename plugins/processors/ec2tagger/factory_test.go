// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)

	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)

	cfg := factory.CreateDefaultConfig()
	setting := processortest.NewNopSettings()

	tProcessor, err := factory.CreateTraces(context.Background(), setting, cfg, consumertest.NewNop())
	assert.Equal(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, tProcessor)

	mProcessor, err := factory.CreateMetrics(context.Background(), setting, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mProcessor)

	lProcessor, err := factory.CreateLogs(context.Background(), setting, cfg, consumertest.NewNop())
	assert.Equal(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, lProcessor)
}
