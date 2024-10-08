// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package jmxfilterprocessor

import (
	"path/filepath"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"gopkg.in/yaml.v2"
)

func TestTranslator(t *testing.T) {
	factory := filterprocessor.NewFactory()

	testCases := map[string]struct {
		input   map[string]any
		index   int
		wantID  string
		want    *confmap.Conf
		wantErr error
	}{
		"ConfigWithNoJmxSet": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"cpu": map[string]any{},
					},
				},
			},
			index:  0,
			wantID: "filter/jmx",
			wantErr: &common.MissingKeyError{
				ID:      component.NewIDWithName(factory.Type(), "jmx"),
				JsonKey: common.ContainerInsightsConfigKey,
			},
		},
		"WithCompleteConfig": {
			input:  testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			index:  0,
			wantID: "filter/jmx",
			want:   testutil.GetConf(t, filepath.Join("testdata", "config.yaml")),
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslatorWithName("jmx")
			require.EqualValues(t, testCase.wantID, tt.ID().String())

			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			require.Equal(t, testCase.wantErr, err)

			if err == nil {
				require.NotNil(t, got)

				gotCfg, ok := got.(*filterprocessor.Config)
				require.True(t, ok)

				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, testCase.want.Unmarshal(wantCfg))

				gotYAML, err := yaml.Marshal(gotCfg)
				require.NoError(t, err)

				wantYAML, err := yaml.Marshal(wantCfg)
				require.NoError(t, err)

				require.Equal(t, wantCfg, gotCfg, "Expected:\n%s\nGot:\n%s", wantYAML, gotYAML)
			}
		})
	}
}
