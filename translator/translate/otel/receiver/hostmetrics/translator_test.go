// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslator(t *testing.T) {
	translator := NewTranslator()
	
	// Test ID
	expectedID := component.NewIDWithName(hostmetricsreceiver.NewFactory().Type(), "")
	assert.Equal(t, expectedID, translator.ID())
	
	// Test translation
	conf := confmap.New()
	cfg, err := translator.Translate(conf)
	require.NoError(t, err)
	
	hostmetricsCfg, ok := cfg.(*hostmetricsreceiver.Config)
	require.True(t, ok)
	
	// Verify memory scraper is configured
	assert.NotNil(t, hostmetricsCfg.Scrapers)
	assert.Contains(t, hostmetricsCfg.Scrapers, getMemoryScraperFactory().Type())
}

func TestTranslatorWithName(t *testing.T) {
	name := "test-name"
	translator := NewTranslatorWithName(name)
	
	expectedID := component.NewIDWithName(hostmetricsreceiver.NewFactory().Type(), name)
	assert.Equal(t, expectedID, translator.ID())
}