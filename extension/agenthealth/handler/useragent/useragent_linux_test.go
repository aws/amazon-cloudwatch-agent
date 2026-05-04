//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package useragent

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/pipelines"
)

func TestSetJournaldFeatureFlags(t *testing.T) {
	tests := []struct {
		name          string
		receivers     map[component.ID]component.Config
		pipelines     pipelines.Config
		expectedFlags []string
	}{
		{
			name:          "no receivers",
			receivers:     map[component.ID]component.Config{},
			expectedFlags: []string{},
		},
		{
			name: "no features",
			receivers: map[component.ID]component.Config{
				component.MustNewID("journald"): &journaldreceiver.JournaldConfig{},
			},
			expectedFlags: []string{},
		},
		{
			name: "default priority not flagged",
			receivers: map[component.ID]component.Config{
				component.MustNewID("journald"): &journaldreceiver.JournaldConfig{
					InputConfig: journald.Config{
						Priority: "info",
					},
				},
			},
			expectedFlags: []string{},
		},
		{
			name: "jd_units",
			receivers: map[component.ID]component.Config{
				component.MustNewID("journald"): &journaldreceiver.JournaldConfig{
					InputConfig: journald.Config{
						Units: []string{"sshd", "kubelet"},
					},
				},
			},
			expectedFlags: []string{flagJournaldUnits},
		},
		{
			name: "jd_priority",
			receivers: map[component.ID]component.Config{
				component.MustNewID("journald"): &journaldreceiver.JournaldConfig{
					InputConfig: journald.Config{
						Priority: "warning",
					},
				},
			},
			expectedFlags: []string{flagJournaldPriority},
		},
		{
			name: "jd_matches",
			receivers: map[component.ID]component.Config{
				component.MustNewID("journald"): &journaldreceiver.JournaldConfig{
					InputConfig: journald.Config{
						Matches: []journald.MatchConfig{{"_SYSTEMD_UNIT": "ssh.service"}},
					},
				},
			},
			expectedFlags: []string{flagJournaldMatches},
		},
		{
			name: "jd_filters",
			receivers: map[component.ID]component.Config{
				component.MustNewID("journald"): &journaldreceiver.JournaldConfig{},
			},
			pipelines: pipelines.Config{
				pipeline.NewIDWithName(pipeline.SignalLogs, "journald/0"): {
					Processors: []component.ID{
						component.NewIDWithName(component.MustNewType("filter"), "journald_0"),
						component.NewIDWithName(component.MustNewType("batch"), "journald_0"),
					},
				},
			},
			expectedFlags: []string{flagJournaldFilters},
		},
		{
			name: "all flags",
			receivers: map[component.ID]component.Config{
				component.MustNewID("journald"): &journaldreceiver.JournaldConfig{
					InputConfig: journald.Config{
						Units:    []string{"sshd"},
						Priority: "err",
						Matches:  []journald.MatchConfig{{"_UID": "0"}},
					},
				},
			},
			pipelines: pipelines.Config{
				pipeline.NewIDWithName(pipeline.SignalLogs, "journald/0"): {
					Processors: []component.ID{
						component.NewIDWithName(component.MustNewType("filter"), "journald_0"),
						component.NewIDWithName(component.MustNewType("batch"), "journald_0"),
					},
				},
			},
			expectedFlags: []string{flagJournaldUnits, flagJournaldPriority, flagJournaldMatches, flagJournaldFilters},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ua := newUserAgent()
			otelCfg := &otelcol.Config{
				Receivers: tt.receivers,
				Service: service.Config{
					Pipelines: tt.pipelines,
				},
			}

			ua.setJournaldFeatureFlags(otelCfg)

			for _, flag := range tt.expectedFlags {
				assert.Contains(t, ua.feature, flag)
			}
			assert.Len(t, ua.feature, len(tt.expectedFlags))
		})
	}
}
