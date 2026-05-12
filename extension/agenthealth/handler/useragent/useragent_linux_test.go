//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
)

func TestSetJournaldFeatureFlags(t *testing.T) {
	tests := []struct {
		name          string
		receivers     map[component.ID]component.Config
		expectedFlags []string
	}{
		{
			name:          "no receivers",
			receivers:     map[component.ID]component.Config{},
			expectedFlags: []string{},
		},
		{
			name: "journald enabled",
			receivers: map[component.ID]component.Config{
				component.MustNewID("journald"): &journaldreceiver.JournaldConfig{},
			},
			expectedFlags: []string{flagJournaldEnabled},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ua := newUserAgent()
			otelCfg := &otelcol.Config{
				Receivers: tt.receivers,
			}

			ua.setJournaldFeatureFlags(otelCfg)

			for _, flag := range tt.expectedFlags {
				assert.Contains(t, ua.feature, flag)
			}
			assert.Len(t, ua.feature, len(tt.expectedFlags))
		})
	}
}
