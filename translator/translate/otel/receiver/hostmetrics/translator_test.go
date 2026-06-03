// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslate(t *testing.T) {
	testCases := map[string]struct {
		input            map[string]interface{}
		nilInput         bool
		expectedInterval time.Duration
	}{
		"NilConf": {
			nilInput:         true,
			expectedInterval: 60 * time.Second,
		},
		"DefaultInterval": {
			input:            map[string]interface{}{},
			expectedInterval: 60 * time.Second,
		},
		"AgentLevelInterval": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"metrics_collection_interval": 30,
				},
			},
			expectedInterval: 30 * time.Second,
		},
		"HostInsightsLevelInterval": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"host_insights": map[string]interface{}{
							"metrics_collection_interval": 10,
						},
					},
				},
			},
			expectedInterval: 10 * time.Second,
		},
		"HostInsightsWinsOverAgent": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"metrics_collection_interval": 30,
				},
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"host_insights": map[string]interface{}{
							"metrics_collection_interval": 10,
						},
					},
				},
			},
			expectedInterval: 10 * time.Second,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var conf *confmap.Conf
			if !tc.nilInput {
				conf = confmap.NewFromStringMap(tc.input)
			}
			cfg, err := NewTranslator().Translate(conf)
			require.NoError(t, err)
			require.NotNil(t, cfg)

			hmCfg, ok := cfg.(*hostMetricsConfig)
			require.True(t, ok)
			assert.Equal(t, tc.expectedInterval, hmCfg.CollectionInterval)
		})
	}
}

func TestTranslateDefaultScrapers(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{})
	cfg, err := NewTranslator().Translate(conf)
	require.NoError(t, err)

	hmCfg := cfg.(*hostMetricsConfig)
	expectedScrapers := []string{"cpu", "disk", "filesystem", "memory", "network", "load", "processes"}
	assert.Equal(t, len(expectedScrapers), len(hmCfg.Scrapers))
	for _, s := range expectedScrapers {
		_, exists := hmCfg.Scrapers[s]
		assert.True(t, exists, "expected scraper %q to be present", s)
	}
}

func TestTranslateWithProcessScraper(t *testing.T) {
	filter := map[string]any{
		"include": map[string]any{
			"match_type": "regexp",
			"names":      []string{"postgres.*"},
		},
		"mute_process_all_errors": true,
	}
	conf := confmap.NewFromStringMap(map[string]interface{}{})
	cfg, err := NewTranslator(WithProcessScraper(filter)).Translate(conf)
	require.NoError(t, err)

	hmCfg := cfg.(*hostMetricsConfig)
	assert.Equal(t, 8, len(hmCfg.Scrapers))
	assert.Equal(t, filter, hmCfg.Scrapers["process"])
}
