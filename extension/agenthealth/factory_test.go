// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()
	assert.Equal(t, &Config{IsUsageDataEnabled: true, Stats: nil}, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreate(t *testing.T) {
	cfg := &Config{}
	got, err := NewFactory().Create(context.Background(), extensiontest.NewNopSettings(component.MustNewType("agenthealth")), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, got)
}
