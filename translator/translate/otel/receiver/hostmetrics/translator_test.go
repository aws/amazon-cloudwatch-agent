// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package hostmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslateLoad(t *testing.T) {
	translator := NewTranslator()
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"metrics": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"load": map[string]interface{}{
					"metrics_collection_interval": 60,
				},
			},
		},
	})

	got, err := translator.Translate(conf)
	assert.NoError(t, err)
	assert.NotNil(t, got)

	configMap, ok := got.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "1m0s", configMap["collection_interval"])

	scrapers, ok := configMap["scrapers"].(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, scrapers, "load")
}

func TestTranslateMissingConfig(t *testing.T) {
	translator := NewTranslator()
	conf := confmap.NewFromStringMap(map[string]interface{}{})

	_, err := translator.Translate(conf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing key")
}

func TestTranslateDefaultInterval(t *testing.T) {
	translator := NewTranslator()
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"metrics": map[string]interface{}{
			"metrics_collected": map[string]interface{}{
				"load": map[string]interface{}{},
			},
		},
	})

	got, err := translator.Translate(conf)
	assert.NoError(t, err)

	configMap := got.(map[string]interface{})
	assert.Equal(t, "1m0s", configMap["collection_interval"])
}