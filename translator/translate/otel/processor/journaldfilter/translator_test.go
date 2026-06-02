// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package journaldfilter

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		translator common.ComponentTranslator
		input      map[string]interface{}
		want       *filterprocessor.Config
		wantErr    bool
	}{
		"WithFilters": {
			translator: NewTranslatorWithFilters("test", []FilterConfig{
				{Type: "exclude", Expression: "error.*"},
				{Type: "include", Expression: "info.*"},
			}),
			input: map[string]interface{}{},
			want: &filterprocessor.Config{
				ErrorMode: "propagate",
				Logs: filterprocessor.LogFilters{
					LogConditions: []string{
						`IsMatch(body, "error.*")`,
						`not IsMatch(body, "info.*")`,
					},
				},
			},
		},
		"MultipleIncludes": {
			translator: NewTranslatorWithFilters("test", []FilterConfig{
				{Type: "include", Expression: "warn.*"},
				{Type: "include", Expression: "error.*"},
			}),
			input: map[string]interface{}{},
			want: &filterprocessor.Config{
				ErrorMode: "propagate",
				Logs: filterprocessor.LogFilters{
					LogConditions: []string{
						`not (IsMatch(body, "warn.*") or IsMatch(body, "error.*"))`,
					},
				},
			},
		},
		"SpecialCharactersEscaped": {
			translator: NewTranslatorWithFilters("test", []FilterConfig{
				{Type: "exclude", Expression: `error.*"sensitive`},
				{Type: "exclude", Expression: `path\\to\\file`},
			}),
			input: map[string]interface{}{},
			want: &filterprocessor.Config{
				ErrorMode: "propagate",
				Logs: filterprocessor.LogFilters{
					LogConditions: []string{
						`IsMatch(body, "error.*\"sensitive")`,
						`IsMatch(body, "path\\\\to\\\\file")`,
					},
				},
			},
		},
		// Mixed exclude+include: filterprocessor drops if ANY condition is true.
		// Exclude conditions are added first, so a log matching both an exclude
		// and include pattern will be dropped (exclude takes precedence).
		// This matches the behavior of file log filters in the legacy Telegraf pipeline.
		"MixedExcludeAndInclude": {
			translator: NewTranslatorWithFilters("test", []FilterConfig{
				{Type: "exclude", Expression: "error.*"},
				{Type: "exclude", Expression: "debug.*"},
				{Type: "include", Expression: "important.*"},
			}),
			input: map[string]interface{}{},
			want: &filterprocessor.Config{
				ErrorMode: "propagate",
				Logs: filterprocessor.LogFilters{
					LogConditions: []string{
						`IsMatch(body, "error.*")`,
						`IsMatch(body, "debug.*")`,
						`not IsMatch(body, "important.*")`,
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tc.input)
			got, err := tc.translator.Translate(conf)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			gotCfg, ok := got.(*filterprocessor.Config)
			require.True(t, ok)
			require.Equal(t, tc.want.ErrorMode, gotCfg.ErrorMode)
			require.Equal(t, tc.want.Logs.LogConditions, gotCfg.Logs.LogConditions)
		})
	}
}
