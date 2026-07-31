// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestResourceAttributesProcessor(t *testing.T) {
	// nil conf -> no processor
	assert.Nil(t, resourceAttributesProcessor(nil))

	// no resource_attributes key -> no processor
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{"collect": map[string]interface{}{"otlp": map[string]interface{}{}}},
	})
	assert.Nil(t, resourceAttributesProcessor(conf))

	// empty map -> no processor
	conf = confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{"resource_attributes": map[string]interface{}{}},
	})
	assert.Nil(t, resourceAttributesProcessor(conf))

	// populated map -> processor present, Translate succeeds
	conf = confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{"resource_attributes": map[string]interface{}{"team": "cloudwatch"}},
	})
	tr := resourceAttributesProcessor(conf)
	require.NotNil(t, tr)
	assert.Equal(t, "resource/opentelemetry", tr.ID().String())
	cfg, err := tr.Translate(conf)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestResourceAttributesProcessor_ReservedKeyRejected(t *testing.T) {
	for _, key := range reservedResourceAttributeKeys {
		t.Run(key, func(t *testing.T) {
			conf := confmap.NewFromStringMap(map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"resource_attributes": map[string]interface{}{key: "/hijacked"},
				},
			})
			tr := resourceAttributesProcessor(conf)
			require.NotNil(t, tr)
			cfg, err := tr.Translate(conf)
			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), "reserved")
		})
	}
}

func TestResourceAttributesProcessor_EmptyKeyRejected(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"resource_attributes": map[string]interface{}{"": "value"},
		},
	})
	tr := resourceAttributesProcessor(conf)
	require.NotNil(t, tr)
	cfg, err := tr.Translate(conf)
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "must not be empty")
}
