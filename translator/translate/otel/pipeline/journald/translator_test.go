// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator_ID(t *testing.T) {
	translator := NewTranslator()
	expected := pipeline.NewIDWithName(pipeline.SignalLogs, "journald")
	assert.Equal(t, expected, translator.ID())
}

func TestTranslator_Translate_MissingKey(t *testing.T) {
	translator := NewTranslator()
	
	// Test with nil config
	_, err := translator.Translate(nil)
	require.Error(t, err)
	assert.IsType(t, &common.MissingKeyError{}, err)

	// Test with empty config
	conf := confmap.New()
	_, err = translator.Translate(conf)
	require.Error(t, err)
	assert.IsType(t, &common.MissingKeyError{}, err)
}

func TestTranslator_Translate_ValidConfig(t *testing.T) {
	translator := NewTranslator()
	
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"journald": map[string]interface{}{
					"units": []interface{}{"nginx.service"},
				},
			},
		},
	})
	
	result, err := translator.Translate(conf)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Verify pipeline components
	assert.Equal(t, 1, result.Receivers.Len())
	assert.Equal(t, 1, result.Processors.Len())
	assert.Equal(t, 1, result.Exporters.Len())
	assert.Equal(t, 2, result.Extensions.Len())
	
	// Verify component IDs
	receiverIDs := result.Receivers.Keys()
	assert.Contains(t, receiverIDs[0].String(), "journald")
	
	processorIDs := result.Processors.Keys()
	assert.Contains(t, processorIDs[0].String(), "batch/journald")
	
	exporterIDs := result.Exporters.Keys()
	assert.Contains(t, exporterIDs[0].String(), "awscloudwatchlogs/journald")
}