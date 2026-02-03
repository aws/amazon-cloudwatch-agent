//go:build linux

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

func TestTranslator(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]interface{}
		wantErr bool
	}{
		{
			name: "missing_key",
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{},
				},
			},
			wantErr: true,
		},
		{
			name: "journald_present",
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"units":    []interface{}{"ssh", "kubelet"},
							"priority": "info",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tt.input)
			translator := NewTranslator()
			require.Equal(t, pipeline.NewIDWithName(pipeline.SignalLogs, "journald"), translator.ID())

			got, err := translator.Translate(conf)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				var missingKeyErr *common.MissingKeyError
				assert.ErrorAs(t, err, &missingKeyErr)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, 1, got.Receivers.Len())
				assert.Equal(t, 1, got.Processors.Len())
				assert.Equal(t, 1, got.Exporters.Len())
				assert.Equal(t, 2, got.Extensions.Len())
			}
		})
	}
}
