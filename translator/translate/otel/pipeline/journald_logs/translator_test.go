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

func TestTranslator(t *testing.T) {
	translator := NewTranslator()
	assert.Equal(t, pipeline.NewIDWithName(pipeline.SignalLogs, common.PipelineNameJournaldLogs), translator.ID())
}

func TestTranslate(t *testing.T) {
	translator := NewTranslator()

	testCases := []struct {
		name    string
		input   map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid_journald_config",
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"directory": "/var/log/journal",
							"units":     []string{"sshd.service"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing_journald_section",
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{},
				},
			},
			wantErr: true,
		},
		{
			name:    "nil_config",
			input:   nil,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var conf *confmap.Conf
			if tc.input != nil {
				conf = confmap.NewFromStringMap(tc.input)
			}

			result, err := translator.Translate(conf)

			if tc.wantErr {
				assert.Error(t, err)
				assert.IsType(t, &common.MissingKeyError{}, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotNil(t, result.Receivers)
			assert.NotNil(t, result.Processors)
			assert.NotNil(t, result.Exporters)
			assert.NotNil(t, result.Extensions)

			// Verify we have the expected components
			assert.Equal(t, 1, result.Receivers.Len())
			assert.Equal(t, 1, result.Processors.Len())
			assert.Equal(t, 1, result.Exporters.Len())
			assert.Equal(t, 2, result.Extensions.Len())
		})
	}
}