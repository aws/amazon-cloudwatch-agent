// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateExporter(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig()
	creationSet := exportertest.NewNopCreateSettings()
	tExporter, err := factory.CreateTracesExporter(context.Background(), creationSet, cfg)
	assert.Equal(t, err, component.ErrDataTypeIsNotSupported)
	assert.Nil(t, tExporter)

	mExporter, err := factory.CreateMetricsExporter(context.Background(), creationSet, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, mExporter)

	tLogs, err := factory.CreateLogsExporter(context.Background(), creationSet, cfg)
	assert.Equal(t, err, component.ErrDataTypeIsNotSupported)
	assert.Nil(t, tLogs)
}
