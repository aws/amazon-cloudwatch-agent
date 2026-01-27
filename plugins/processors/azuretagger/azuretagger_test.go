// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azuretagger

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudmetadata"
)

func TestNewTagger(t *testing.T) {
	cfg := &Config{
		AzureMetadataTags:    []string{"InstanceId"},
		AzureInstanceTagKeys: []string{"Environment"},
	}
	logger := zap.NewNop()

	tagger := newTagger(cfg, logger)

	if tagger == nil {
		t.Fatal("newTagger returned nil")
	}
	if tagger.Config != cfg {
		t.Error("Config not set correctly")
	}
	if tagger.logger != logger {
		t.Error("Logger not set correctly")
	}
}

func TestTagger_Start_NoProvider(t *testing.T) {
	// Reset global provider
	cloudmetadata.ResetGlobalProvider()

	cfg := &Config{}
	logger := zap.NewNop()
	tagger := newTagger(cfg, logger)

	err := tagger.Start(context.Background(), nil)
	if err != nil {
		t.Errorf("Start() error = %v, want nil", err)
	}

	if !tagger.started {
		t.Error("Tagger should be started even without provider")
	}
}

func TestTagger_Start_NonAzureProvider(t *testing.T) {
	// Set up mock provider for AWS
	mockProvider := &cloudmetadata.MockProvider{
		CloudProvider: cloudmetadata.CloudProviderAWS,
	}
	cloudmetadata.SetGlobalProviderForTest(mockProvider)
	defer cloudmetadata.ResetGlobalProvider()

	cfg := &Config{}
	logger := zap.NewNop()
	tagger := newTagger(cfg, logger)

	err := tagger.Start(context.Background(), nil)
	if err != nil {
		t.Errorf("Start() error = %v, want nil", err)
	}

	if !tagger.started {
		t.Error("Tagger should be started even on non-Azure")
	}
}

func TestTagger_Start_AzureProvider(t *testing.T) {
	// Set up mock provider for Azure
	mockProvider := &cloudmetadata.MockProvider{
		CloudProvider:    cloudmetadata.CloudProviderAzure,
		InstanceID:       "test-vm-id",
		InstanceType:     "Standard_D2s_v3",
		Region:           "eastus",
		AccountID:        "test-subscription",
		ScalingGroupName: "test-vmss",
		ResourceGroup:    "test-rg",
		Tags: map[string]string{
			"Environment": "Production",
			"Team":        "Engineering",
		},
	}
	cloudmetadata.SetGlobalProviderForTest(mockProvider)
	defer cloudmetadata.ResetGlobalProvider()

	cfg := &Config{
		AzureMetadataTags:    []string{"InstanceId", "InstanceType", "VMScaleSetName"},
		AzureInstanceTagKeys: []string{"Environment"},
	}
	logger := zap.NewNop()
	tagger := newTagger(cfg, logger)

	err := tagger.Start(context.Background(), nil)
	if err != nil {
		t.Errorf("Start() error = %v, want nil", err)
	}

	if !tagger.started {
		t.Error("Tagger should be started")
	}

	// Verify metadata was captured
	if tagger.azureMetadataRespond.instanceID != "test-vm-id" {
		t.Errorf("instanceID = %q, want %q", tagger.azureMetadataRespond.instanceID, "test-vm-id")
	}

	// Verify tags were captured
	if tagger.azureTagCache["Environment"] != "Production" {
		t.Errorf("Environment tag = %q, want %q", tagger.azureTagCache["Environment"], "Production")
	}

	// Team tag should NOT be captured (not in AzureInstanceTagKeys)
	if _, ok := tagger.azureTagCache["Team"]; ok {
		t.Error("Team tag should not be captured")
	}
}

func TestTagger_Start_AllTags(t *testing.T) {
	mockProvider := &cloudmetadata.MockProvider{
		CloudProvider: cloudmetadata.CloudProviderAzure,
		InstanceID:    "test-vm-id",
		Tags: map[string]string{
			"Environment": "Production",
			"Team":        "Engineering",
			"CostCenter":  "12345",
		},
	}
	cloudmetadata.SetGlobalProviderForTest(mockProvider)
	defer cloudmetadata.ResetGlobalProvider()

	cfg := &Config{
		AzureInstanceTagKeys: []string{"*"},
	}
	logger := zap.NewNop()
	tagger := newTagger(cfg, logger)

	err := tagger.Start(context.Background(), nil)
	if err != nil {
		t.Errorf("Start() error = %v, want nil", err)
	}

	// All tags should be captured
	if len(tagger.azureTagCache) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(tagger.azureTagCache))
	}
}

func TestTagger_ProcessMetrics(t *testing.T) {
	mockProvider := &cloudmetadata.MockProvider{
		CloudProvider: cloudmetadata.CloudProviderAzure,
		InstanceID:    "test-vm-id",
		InstanceType:  "Standard_D2s_v3",
		Tags: map[string]string{
			"Environment": "Production",
		},
	}
	cloudmetadata.SetGlobalProviderForTest(mockProvider)
	defer cloudmetadata.ResetGlobalProvider()

	cfg := &Config{
		AzureMetadataTags:    []string{"InstanceId", "InstanceType"},
		AzureInstanceTagKeys: []string{"Environment"},
	}
	logger := zap.NewNop()
	tagger := newTagger(cfg, logger)

	err := tagger.Start(context.Background(), nil)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Create test metrics
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("test_metric")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(42.0)

	// Process metrics
	result, err := tagger.processMetrics(context.Background(), md)
	if err != nil {
		t.Fatalf("processMetrics() error = %v", err)
	}

	// Verify attributes were added
	attrs := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes()

	val, ok := attrs.Get("InstanceId")
	if !ok || val.Str() != "test-vm-id" {
		t.Errorf("InstanceId = %v, want %q", val, "test-vm-id")
	}

	val, ok = attrs.Get("InstanceType")
	if !ok || val.Str() != "Standard_D2s_v3" {
		t.Errorf("InstanceType = %v, want %q", val, "Standard_D2s_v3")
	}

	val, ok = attrs.Get("Environment")
	if !ok || val.Str() != "Production" {
		t.Errorf("Environment = %v, want %q", val, "Production")
	}
}

func TestTagger_ProcessMetrics_NotStarted(t *testing.T) {
	cfg := &Config{}
	logger := zap.NewNop()
	tagger := newTagger(cfg, logger)
	// Don't call Start()

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("test_metric")
	m.SetEmptyGauge().DataPoints().AppendEmpty()

	result, err := tagger.processMetrics(context.Background(), md)
	if err != nil {
		t.Fatalf("processMetrics() error = %v", err)
	}

	// Should return empty metrics when not started
	if result.ResourceMetrics().Len() != 0 {
		t.Error("Expected empty metrics when not started")
	}
}

func TestTagger_Shutdown(t *testing.T) {
	cfg := &Config{}
	logger := zap.NewNop()
	tagger := newTagger(cfg, logger)

	// Start first
	cloudmetadata.ResetGlobalProvider()
	err := tagger.Start(context.Background(), nil)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Shutdown
	err = tagger.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown() error = %v, want nil", err)
	}
}

func TestTagger_UpdateOtelAttributes_ExistingAttributes(t *testing.T) {
	tagger := &Tagger{
		Config: &Config{},
		logger: zap.NewNop(),
		azureTagCache: map[string]string{
			"Environment": "Production",
		},
		azureMetadataLookup: azureMetadataLookupType{
			instanceID: true,
		},
		azureMetadataRespond: azureMetadataRespondType{
			instanceID: "test-vm-id",
		},
		started: true,
	}

	// Create attributes with existing values
	attrs := pcommon.NewMap()
	attrs.PutStr("InstanceId", "existing-id")
	attrs.PutStr("Environment", "existing-env")

	tagger.updateOtelAttributes([]pcommon.Map{attrs})

	// Existing values should NOT be overwritten
	val, _ := attrs.Get("InstanceId")
	if val.Str() != "existing-id" {
		t.Errorf("InstanceId was overwritten: got %q, want %q", val.Str(), "existing-id")
	}

	val, _ = attrs.Get("Environment")
	if val.Str() != "existing-env" {
		t.Errorf("Environment was overwritten: got %q, want %q", val.Str(), "existing-env")
	}
}

func TestTagger_UpdateOtelAttributes_RemovesHost(t *testing.T) {
	tagger := &Tagger{
		Config:        &Config{},
		logger:        zap.NewNop(),
		azureTagCache: map[string]string{},
		started:       true,
	}

	attrs := pcommon.NewMap()
	attrs.PutStr("host", "test-host")

	tagger.updateOtelAttributes([]pcommon.Map{attrs})

	if _, ok := attrs.Get("host"); ok {
		t.Error("host attribute should be removed")
	}
}

func TestGetOtelAttributes_AllMetricTypes(t *testing.T) {
	tests := []struct {
		name    string
		setupFn func(m pmetric.Metric)
		wantLen int
	}{
		{
			name: "Gauge",
			setupFn: func(m pmetric.Metric) {
				m.SetEmptyGauge().DataPoints().AppendEmpty()
				m.Gauge().DataPoints().AppendEmpty()
			},
			wantLen: 2,
		},
		{
			name: "Sum",
			setupFn: func(m pmetric.Metric) {
				m.SetEmptySum().DataPoints().AppendEmpty()
			},
			wantLen: 1,
		},
		{
			name: "Histogram",
			setupFn: func(m pmetric.Metric) {
				m.SetEmptyHistogram().DataPoints().AppendEmpty()
			},
			wantLen: 1,
		},
		{
			name: "ExponentialHistogram",
			setupFn: func(m pmetric.Metric) {
				m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pmetric.NewMetric()
			tt.setupFn(m)

			attrs := getOtelAttributes(m)
			if len(attrs) != tt.wantLen {
				t.Errorf("getOtelAttributes() returned %d attributes, want %d", len(attrs), tt.wantLen)
			}
		})
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "<empty>"},
		{"abc", "<present>"},
		{"abcd", "<present>"},
		{"abcde", "abcd..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := maskValue(tt.input)
			if got != tt.want {
				t.Errorf("maskValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTagger_DeriveAzureMetadataFromProvider_UnsupportedKey(t *testing.T) {
	mockProvider := &cloudmetadata.MockProvider{
		CloudProvider: cloudmetadata.CloudProviderAzure,
		InstanceID:    "test-vm-id",
	}

	cfg := &Config{
		AzureMetadataTags: []string{"UnsupportedKey", "InstanceId"},
	}
	logger := zap.NewNop()
	tagger := newTagger(cfg, logger)
	tagger.shutdownC = make(chan bool)

	err := tagger.deriveAzureMetadataFromProvider(mockProvider)
	if err != nil {
		t.Errorf("deriveAzureMetadataFromProvider() error = %v", err)
	}

	// InstanceId should still be captured
	if !tagger.azureMetadataLookup.instanceID {
		t.Error("instanceID lookup should be enabled")
	}
}

func TestTagger_RefreshLoop_Shutdown(t *testing.T) {
	mockProvider := &cloudmetadata.MockProvider{
		CloudProvider: cloudmetadata.CloudProviderAzure,
		InstanceID:    "test-vm-id",
		Tags:          map[string]string{"Key": "Value"},
	}

	cfg := &Config{
		RefreshTagsInterval:  50 * time.Millisecond,
		AzureInstanceTagKeys: []string{"*"},
	}
	logger := zap.NewNop()
	tagger := newTagger(cfg, logger)
	tagger.shutdownC = make(chan bool)
	tagger.azureTagCache = make(map[string]string)
	tagger.useAllTags = true

	// Start refresh loop in goroutine
	done := make(chan bool)
	go func() {
		tagger.refreshLoopTags(mockProvider)
		done <- true
	}()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	close(tagger.shutdownC)

	// Wait for goroutine to exit
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Refresh loop did not stop after shutdown")
	}
}
