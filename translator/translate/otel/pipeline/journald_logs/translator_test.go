// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald_logs

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
	expected := pipeline.NewIDWithName(pipeline.SignalLogs, common.PipelineNameJournaldLogs)
	assert.Equal(t, expected, translator.ID())
}

func TestTranslator_Translate_MissingConfig(t *testing.T) {
	translator := NewTranslator()
	
	// Test with nil config
	result, err := translator.Translate(nil)
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing key")
	
	// Test with empty config
	conf := confmap.New()
	result, err = translator.Translate(conf)
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing key")
}

func TestTranslator_Translate_ValidConfig(t *testing.T) {
	translator := NewTranslator()
	
	// Create config with journald section
	configMap := map[string]interface{}{
		"logs": map[string]interface{}{
			"logs_collected": map[string]interface{}{
				"journald": map[string]interface{}{
					"collect_list": []interface{}{
						map[string]interface{}{
							"log_group_name":  "test-group",
							"log_stream_name": "test-stream",
						},
					},
				},
			},
		},
	}
	
	conf := confmap.NewFromStringMap(configMap)
	result, err := translator.Translate(conf)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Verify all components are present
	assert.Len(t, result.Receivers.Keys(), 1)
	assert.Len(t, result.Processors.Keys(), 2) // filter + batch
	assert.Len(t, result.Exporters.Keys(), 1)
	assert.Len(t, result.Extensions.Keys(), 2) // 2 agenthealth extensions
	
	// Verify component types by converting IDs to strings
	receiverKeys := make([]string, len(result.Receivers.Keys()))
	for i, key := range result.Receivers.Keys() {
		receiverKeys[i] = key.String()
	}
	processorKeys := make([]string, len(result.Processors.Keys()))
	for i, key := range result.Processors.Keys() {
		processorKeys[i] = key.String()
	}
	exporterKeys := make([]string, len(result.Exporters.Keys()))
	for i, key := range result.Exporters.Keys() {
		exporterKeys[i] = key.String()
	}

	assert.Contains(t, receiverKeys, "journald")
	assert.Contains(t, processorKeys, "filter")
	assert.Contains(t, processorKeys, "batch/journald_logs")
	assert.Contains(t, exporterKeys, "awscloudwatchlogs/journald_logs")
}