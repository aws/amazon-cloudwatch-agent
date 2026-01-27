// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azuretagger

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()

	if factory == nil {
		t.Fatal("NewFactory() returned nil")
	}

	if factory.Type() != TypeStr {
		t.Errorf("Type() = %v, want %v", factory.Type(), TypeStr)
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	if cfg == nil {
		t.Fatal("CreateDefaultConfig() returned nil")
	}

	config, ok := cfg.(*Config)
	if !ok {
		t.Fatal("CreateDefaultConfig() did not return *Config")
	}

	if config.RefreshTagsInterval != 0 {
		t.Errorf("RefreshTagsInterval = %v, want 0", config.RefreshTagsInterval)
	}

	if len(config.AzureMetadataTags) != 0 {
		t.Errorf("AzureMetadataTags = %v, want empty", config.AzureMetadataTags)
	}

	if len(config.AzureInstanceTagKeys) != 0 {
		t.Errorf("AzureInstanceTagKeys = %v, want empty", config.AzureInstanceTagKeys)
	}

	// Verify config struct is valid
	if err := componenttest.CheckConfigStruct(cfg); err != nil {
		t.Errorf("CheckConfigStruct() error = %v", err)
	}
}

func TestCreateMetricsProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := processortest.NewNopSettings(component.MustNewType("azuretagger"))
	mp, err := factory.CreateMetrics(context.Background(), set, cfg, consumertest.NewNop())

	if err != nil {
		t.Fatalf("CreateMetrics() error = %v", err)
	}

	if mp == nil {
		t.Fatal("CreateMetrics() returned nil processor")
	}
}

func TestCreateMetricsProcessor_InvalidConfig(t *testing.T) {
	factory := NewFactory()

	set := processortest.NewNopSettings(component.MustNewType("azuretagger"))
	// Pass wrong config type
	_, err := factory.CreateMetrics(context.Background(), set, "invalid", consumertest.NewNop())

	if err == nil {
		t.Error("CreateMetrics() with invalid config should return error")
	}
}

func TestCreateTracesProcessor_NotSupported(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := processortest.NewNopSettings(component.MustNewType("azuretagger"))
	tp, err := factory.CreateTraces(context.Background(), set, cfg, consumertest.NewNop())

	if err != pipeline.ErrSignalNotSupported {
		t.Errorf("CreateTraces() error = %v, want %v", err, pipeline.ErrSignalNotSupported)
	}
	if tp != nil {
		t.Error("CreateTraces() should return nil processor")
	}
}

func TestCreateLogsProcessor_NotSupported(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := processortest.NewNopSettings(component.MustNewType("azuretagger"))
	lp, err := factory.CreateLogs(context.Background(), set, cfg, consumertest.NewNop())

	if err != pipeline.ErrSignalNotSupported {
		t.Errorf("CreateLogs() error = %v, want %v", err, pipeline.ErrSignalNotSupported)
	}
	if lp != nil {
		t.Error("CreateLogs() should return nil processor")
	}
}

func TestTypeStr(t *testing.T) {
	if TypeStr.String() != "azuretagger" {
		t.Errorf("TypeStr = %q, want %q", TypeStr.String(), "azuretagger")
	}
}
