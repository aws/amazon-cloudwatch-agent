// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windowseventlog

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/filestorage"
)

func TestTranslator_Translate(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		channel  string
		raw      bool
		resource map[string]string
	}{
		{
			name:    "Structured",
			id:      "system_0",
			channel: "System",
			raw:     false,
			resource: map[string]string{
				"aws.log.source":     "windows_events",
				"aws.log.group.name": "/aws/windows/System",
			},
		},
		{
			name:    "Raw",
			id:      "system_0",
			channel: "System",
			raw:     true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := NewTranslator(tc.id, tc.channel, tc.raw, tc.resource)
			assert.Equal(t, "windowseventlog/"+tc.id, tr.ID().String())

			cfg, err := tr.Translate(nil)
			require.NoError(t, err)

			got, ok := cfg.(*windowseventlogreceiver.WindowsLogConfig)
			require.True(t, ok)
			assert.Equal(t, tc.channel, got.InputConfig.Channel)
			assert.Equal(t, "end", got.InputConfig.StartAt)
			assert.Equal(t, tc.raw, got.InputConfig.Raw)
			assert.NotNil(t, got.StorageID)
			assert.Equal(t, filestorage.ComponentID(), *got.StorageID)
			if len(tc.resource) > 0 {
				for k, v := range tc.resource {
					assert.Equal(t, helper.ExprStringConfig(v), got.InputConfig.Resource[k])
				}
			} else {
				assert.Empty(t, got.InputConfig.Resource)
			}
		})
	}
}
