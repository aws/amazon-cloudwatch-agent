// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rollupprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, component.MustNewType(typeStr), factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, &Config{CacheSize: defaultCacheSize}, cfg)
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()
	mp, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), nil, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, mp)

	cfg := factory.CreateDefaultConfig().(*Config)
	mp, err = factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mp)

	assert.NoError(t, mp.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, mp.Shutdown(context.Background()))
}
