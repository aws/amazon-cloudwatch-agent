// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package journald

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/filestorage"
)

func TestTranslatorWithConfig(t *testing.T) {
	testCases := map[string]struct {
		units    []string
		priority string
		matches  []journald.MatchConfig
		mode     string
		want     *journaldreceiver.JournaldConfig
		wantMode string
		wantErr  bool
	}{
		"WithAllFields": {
			units:    []string{"sshd", "systemd"},
			priority: "err",
			matches:  []journald.MatchConfig{{"_PID": "1"}},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: journald.Config{
					Units:    []string{"sshd", "systemd"},
					Priority: "err",
					Matches:  []journald.MatchConfig{{"_PID": "1"}},
				},
			},
			// Absent mode preserves the contrib default (journalctl).
			wantMode: journaldreceiver.ModeJournalctl,
		},
		"WithDefaultPriority": {
			units: nil,
			want: &journaldreceiver.JournaldConfig{
				InputConfig: journald.Config{
					Priority: "info",
				},
			},
			wantMode: journaldreceiver.ModeJournalctl,
		},
		"WithUnitsOnly": {
			units: []string{"sshd"},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: journald.Config{
					Units:    []string{"sshd"},
					Priority: "info",
				},
			},
			wantMode: journaldreceiver.ModeJournalctl,
		},
		"WithEmptyModePreservesDefault": {
			mode: "",
			want: &journaldreceiver.JournaldConfig{
				InputConfig: journald.Config{Priority: "info"},
			},
			wantMode: journaldreceiver.ModeJournalctl,
		},
		"WithNativeMode": {
			mode: "native",
			want: &journaldreceiver.JournaldConfig{
				InputConfig: journald.Config{Priority: "info"},
			},
			wantMode: journaldreceiver.ModeNative,
		},
		"WithJournalctlMode": {
			mode: "journalctl",
			want: &journaldreceiver.JournaldConfig{
				InputConfig: journald.Config{Priority: "info"},
			},
			wantMode: journaldreceiver.ModeJournalctl,
		},
		"WithInvalidMode": {
			mode:    "bogus",
			wantErr: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			translator := NewTranslatorWithConfig("test", tc.units, tc.priority, tc.matches, tc.mode)
			got, err := translator.Translate(nil)

			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)

			gotCfg, ok := got.(*journaldreceiver.JournaldConfig)
			require.True(t, ok)

			assert.Equal(t, tc.want.InputConfig.Units, gotCfg.InputConfig.Units)
			assert.Equal(t, tc.want.InputConfig.Priority, gotCfg.InputConfig.Priority)
			assert.Equal(t, tc.want.InputConfig.Matches, gotCfg.InputConfig.Matches)
			assert.Equal(t, tc.wantMode, gotCfg.Mode)
			assert.NotNil(t, gotCfg.StorageID)
			assert.Equal(t, filestorage.ComponentID(), *gotCfg.StorageID)
		})
	}
}

func TestTranslatorID(t *testing.T) {
	translator := NewTranslatorWithConfig("journald_0", nil, "", nil, "")
	assert.Equal(t, "journald/journald_0", translator.ID().String())
}
