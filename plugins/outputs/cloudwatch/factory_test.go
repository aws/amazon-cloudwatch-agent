// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pipeline"
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
	creationSet := exportertest.NewNopSettings(component.MustNewType("awscloudwatch"))
	tExporter, err := factory.CreateTraces(context.Background(), creationSet, cfg)
	assert.Equal(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, tExporter)

	mExporter, err := factory.CreateMetrics(context.Background(), creationSet, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, mExporter)

	tLogs, err := factory.CreateLogs(context.Background(), creationSet, cfg)
	assert.Equal(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, tLogs)
}
