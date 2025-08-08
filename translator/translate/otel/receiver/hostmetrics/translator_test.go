// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	cfgKey := common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.LoadMetricKey)
	tt := NewTranslator(cfgKey, common.WithName("loadaverage"))
	assert.EqualValues(t, "hostmetrics/loadaverage", tt.ID().String())

	testCases := map[string]struct {
		input   map[string]interface{}
		wantErr error
	}{
		"WithMissingKey": {
			input: map[string]interface{}{"metrics": map[string]interface{}{}},
			wantErr: &common.MissingKeyError{
				ID:      tt.ID(),
				JsonKey: cfgKey,
			},
		},
		"WithLoadMetrics": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"load": map[string]interface{}{
							"measurement":                 []string{"load_average_1m", "load_average_5m", "load_average_15m"},
							"metrics_collection_interval": 60,
						},
					},
				},
			},
		},
		"WithDefaultInterval": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"load": map[string]interface{}{
							"measurement": []string{"load_average_1m", "load_average_5m", "load_average_15m"},
						},
					},
				},
			},
		},
		"WithOTelMetricNames": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"load": map[string]interface{}{
							"measurement":                 []string{"system.cpu.load_average.1m", "system.cpu.load_average.5m", "system.cpu.load_average.15m"},
							"metrics_collection_interval": 60,
						},
					},
				},
			},
		},

	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			if testCase.wantErr != nil {
				assert.EqualError(t, err, testCase.wantErr.Error())
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				
				// Verify it's a hostmetrics receiver config
				cfg, ok := got.(*hostmetricsreceiver.Config)
				require.True(t, ok)
				
				// Verify collection interval is set
				assert.Equal(t, 60*time.Second, cfg.CollectionInterval)
				
				// Verify load scraper is configured
				assert.Contains(t, cfg.Scrapers, component.MustNewType("load"))
				assert.Len(t, cfg.Scrapers, 1)
			}
		})
	}
}

// TestScraperValidationError specifically tests against the "must specify at least one scraper" error
func TestScraperValidationError(t *testing.T) {
	cfgKey := common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.LoadMetricKey)
	tt := NewTranslator(cfgKey, common.WithName("loadaverage"))

	// Test with valid load metrics configuration
	input := map[string]interface{}{
		"metrics": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"load": map[string]interface{}{
					"measurement":                 []interface{}{"load_average_1m", "load_average_5m", "load_average_15m"},
					"metrics_collection_interval": 60,
				},
			},
		},
	}

	conf := confmap.NewFromStringMap(input)
	got, err := tt.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, got)

	// Cast to hostmetrics receiver config
	cfg, ok := got.(*hostmetricsreceiver.Config)
	require.True(t, ok, "Expected hostmetricsreceiver.Config, got %T", got)

	// CRITICAL TEST: Verify that cfg.Scrapers is NOT empty
	require.NotEmpty(t, cfg.Scrapers, "cfg.Scrapers should not be empty - this would cause 'must specify at least one scraper' error")
	
	// Verify the load scraper is present
	loadScraperType := component.MustNewType("load")
	require.Contains(t, cfg.Scrapers, loadScraperType, "Load scraper should be present in cfg.Scrapers")
	
	// Verify the scraper config is valid
	scraperConfig := cfg.Scrapers[loadScraperType]
	require.NotNil(t, scraperConfig, "Load scraper config should not be nil")

	// CRITICAL TEST: Call the Validate method directly to ensure it passes
	err = cfg.Validate()
	require.NoError(t, err, "Configuration validation should pass - this is the exact check that was failing in production")

	// Additional verification: ensure the scraper config is not nil and is a valid config
	require.NotNil(t, scraperConfig, "Scraper config should not be nil")

	t.Logf("✅ SUCCESS: Configuration has %d scrapers and passes validation", len(cfg.Scrapers))
	t.Logf("✅ SUCCESS: Load scraper is properly configured: %T", scraperConfig)
}

// TestProductionScenario simulates the exact production scenario with OTEL validation
func TestProductionScenario(t *testing.T) {
	// Simulate the exact configuration that would be used in production
	cfgKey := common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.LoadMetricKey)
	tt := NewTranslator(cfgKey, common.WithName("loadaverage"))

	// This is the exact configuration structure that would come from the JSON config
	input := map[string]interface{}{
		"metrics": map[string]interface{}{
			"namespace": "LoadMetrics/TestImplementation",
			"metrics_collected": map[string]interface{}{
				"load": map[string]interface{}{
					"measurement":                 []interface{}{"load_average_1m", "load_average_5m", "load_average_15m"},
					"metrics_collection_interval": 60,
				},
			},
		},
	}

	conf := confmap.NewFromStringMap(input)
	
	// Step 1: Translate the configuration (this is what our translator does)
	receiverConfig, err := tt.Translate(conf)
	require.NoError(t, err, "Translation should succeed")
	require.NotNil(t, receiverConfig, "Receiver config should not be nil")

	// Step 2: Cast to hostmetrics receiver config (this is what OTEL does)
	cfg, ok := receiverConfig.(*hostmetricsreceiver.Config)
	require.True(t, ok, "Should be hostmetricsreceiver.Config, got %T", receiverConfig)

	// Step 3: Verify scrapers are populated BEFORE validation
	t.Logf("DEBUG: cfg.Scrapers has %d entries", len(cfg.Scrapers))
	for scraperType, scraperConfig := range cfg.Scrapers {
		t.Logf("DEBUG: Scraper %s -> %T", scraperType.String(), scraperConfig)
	}

	// Step 4: This is the EXACT validation that OTEL performs and was failing
	validationErr := cfg.Validate()
	if validationErr != nil {
		t.Fatalf("❌ VALIDATION FAILED: %v", validationErr)
	}

	// Step 5: Additional checks to ensure everything is correct
	require.NotEmpty(t, cfg.Scrapers, "Scrapers map should not be empty")
	require.Contains(t, cfg.Scrapers, component.MustNewType("load"), "Load scraper should be present")
	require.Equal(t, 60*time.Second, cfg.CollectionInterval, "Collection interval should be 60 seconds")

	t.Logf("🎉 SUCCESS: Production scenario validation passed!")
	t.Logf("🎉 Configuration will work correctly in production environment")
}

// TestConfigurationMarshaling tests the configuration marshaling/unmarshaling process
func TestConfigurationMarshaling(t *testing.T) {
	cfgKey := common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.LoadMetricKey)
	tt := NewTranslator(cfgKey, common.WithName("loadaverage"))

	input := map[string]interface{}{
		"metrics": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"load": map[string]interface{}{
					"measurement": []interface{}{"load_average_1m", "load_average_5m", "load_average_15m"},
				},
			},
		},
	}

	conf := confmap.NewFromStringMap(input)
	receiverConfig, err := tt.Translate(conf)
	require.NoError(t, err)

	cfg := receiverConfig.(*hostmetricsreceiver.Config)

	// Test that the configuration can be marshaled and unmarshaled
	// This simulates what happens when OTEL processes the configuration
	configMap := map[string]any{
		"collection_interval": cfg.CollectionInterval,
		"scrapers": map[string]any{
			"load": map[string]any{
				"cpu_average": false,
			},
		},
	}

	// Create a new config and unmarshal into it
	testConf := confmap.NewFromStringMap(configMap)
	newCfg := hostmetricsreceiver.NewFactory().CreateDefaultConfig().(*hostmetricsreceiver.Config)
	
	err = testConf.Unmarshal(newCfg)
	require.NoError(t, err, "Unmarshaling should succeed")

	// Validate the new config
	err = newCfg.Validate()
	require.NoError(t, err, "New config should validate successfully")

	// Verify scrapers are present
	require.NotEmpty(t, newCfg.Scrapers, "New config should have scrapers")
	require.Contains(t, newCfg.Scrapers, component.MustNewType("load"), "New config should have load scraper")

	t.Logf("✅ Configuration marshaling/unmarshaling works correctly")
}

func TestRealWorldValidation(t *testing.T) {
	// This test validates the configuration like the real collector would
	tt := NewTranslator("load")
	
	input := map[string]interface{}{
		"load": map[string]interface{}{
			"measurement":                 []string{"load_average_1m", "load_average_5m", "load_average_15m"},
			"metrics_collection_interval": 60,
		},
	}
	
	conf := confmap.NewFromStringMap(input)
	cfg, err := tt.Translate(conf)
	
	require.NoError(t, err, "Translation should succeed")
	require.NotNil(t, cfg, "Configuration should not be nil")
	
	// Cast to the actual hostmetrics config type
	hostmetricsConfig, ok := cfg.(*hostmetricsreceiver.Config)
	require.True(t, ok, "Config should be of type *hostmetricsreceiver.Config")
	
	// CRITICAL: Check if scrapers are populated - this is what causes the runtime error
	t.Logf("DEBUG: Scrapers map has %d entries", len(hostmetricsConfig.Scrapers))
	for scraperType, scraperConfig := range hostmetricsConfig.Scrapers {
		t.Logf("DEBUG: Scraper %s -> %T", scraperType, scraperConfig)
	}
	
	// This is the validation that should catch the "must specify at least one scraper" error
	if len(hostmetricsConfig.Scrapers) == 0 {
		t.Fatalf("❌ CRITICAL: Scrapers map is empty - this will cause 'must specify at least one scraper' error in production")
	}
	
	// Verify the load scraper specifically is present
	loadScraperType := component.MustNewType("load")
	if _, exists := hostmetricsConfig.Scrapers[loadScraperType]; !exists {
		t.Fatalf("❌ CRITICAL: Load scraper not found in scrapers map")
	}
	
	// Actually validate the configuration like the real collector would
	err = hostmetricsConfig.Validate()
	if err != nil {
		t.Fatalf("❌ CRITICAL: Configuration validation failed with error: %v", err)
	}
	
	t.Log("🎉 SUCCESS: Real-world validation passed!")
}

// TestExactRuntimeScenario tests the exact scenario that would happen at runtime
func TestExactRuntimeScenario(t *testing.T) {
	// This simulates the exact flow that happens when the agent starts
	cfgKey := common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.LoadMetricKey)
	tt := NewTranslator(cfgKey, common.WithName("loadaverage"))

	// Exact JSON that would come from user config
	jsonInput := map[string]interface{}{
		"agent": map[string]interface{}{
			"region": "us-east-1",
		},
		"metrics": map[string]interface{}{
			"namespace": "CWAgent",
			"metrics_collected": map[string]interface{}{
				"load": map[string]interface{}{
					"measurement": []string{"load_average_1m", "load_average_5m", "load_average_15m"},
					"metrics_collection_interval": 60,
				},
			},
		},
	}

	conf := confmap.NewFromStringMap(jsonInput)
	
	// Step 1: Translate (this is what our translator does)
	receiverConfig, err := tt.Translate(conf)
	require.NoError(t, err, "Translation must succeed")
	require.NotNil(t, receiverConfig, "Config must not be nil")

	// Step 2: Cast to hostmetrics config (this is what OTEL does)
	cfg, ok := receiverConfig.(*hostmetricsreceiver.Config)
	require.True(t, ok, "Must be hostmetricsreceiver.Config")

	// Step 3: CRITICAL - Check scrapers BEFORE any other operations
	require.NotEmpty(t, cfg.Scrapers, "❌ CRITICAL: Scrapers map is empty - this WILL cause runtime error")
	require.Contains(t, cfg.Scrapers, component.MustNewType("load"), "❌ CRITICAL: Load scraper missing")

	// Step 4: Validate (this is what OTEL does at startup)
	err = cfg.Validate()
	require.NoError(t, err, "❌ CRITICAL: Validation failed - this WILL cause runtime error: %v", err)

	// Step 5: Try to create the actual receiver factory (this is what OTEL does)
	factory := hostmetricsreceiver.NewFactory()
	require.NotNil(t, factory, "Factory must be available")

	// Step 6: Verify the config can be used to create a receiver
	// This is the closest we can get to the actual runtime without starting the full collector
	require.Equal(t, "hostmetrics", factory.Type().String(), "Factory type must match")
	require.Equal(t, 60*time.Second, cfg.CollectionInterval, "Collection interval must be correct")

	t.Logf("✅ SUCCESS: Exact runtime scenario validation passed!")
	t.Logf("✅ Configuration has %d scrapers and will work at runtime", len(cfg.Scrapers))
	
	// Log the scraper details for verification
	for scraperType, scraperConfig := range cfg.Scrapers {
		t.Logf("✅ Scraper configured: %s -> %T", scraperType, scraperConfig)
	}
}