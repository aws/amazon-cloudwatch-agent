// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

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
		"DefaultConfig": {
			translator: NewTranslator(),
			input:      map[string]interface{}{},
			want: &filterprocessor.Config{
				ErrorMode: "ignore",
			},
		},
		"WithFilters": {
			translator: NewTranslatorWithFilters("test", []FilterConfig{
				{Type: "exclude", Expression: "error.*"},
				{Type: "include", Expression: "info.*"},
			}),
			input: map[string]interface{}{},
			want: &filterprocessor.Config{
				ErrorMode: "ignore",
				Logs: filterprocessor.LogFilters{
					LogConditions: []string{
						`IsMatch(body, "error.*")`,
						`not IsMatch(body, "info.*")`,
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