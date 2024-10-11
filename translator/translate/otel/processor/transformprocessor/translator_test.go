// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package transformprocessor

import (
	_ "embed"
	"path/filepath"
	"sort"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	factory := transformprocessor.NewFactory()

	testCases := map[string]struct {
		translator common.Translator[component.Config]
		input      map[string]any
		index      int
		wantID     string
		want       string
		wantErr    error
	}{
		"NoContainerInsights": {
			input: map[string]any{},
			wantErr: &common.MissingKeyError{
				ID:      component.NewIDWithName(factory.Type(), "jmx"),
				JsonKey: common.ContainerInsightsConfigKey,
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslatorWithName("jmx")

			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			require.Equal(t, testCase.wantErr, err)

			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*transformprocessor.Config)

				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				yamlConfig, err := common.GetYamlFileToYamlConfig(wantCfg, testCase.want)
				require.NoError(t, err)
				assert.Equal(t, yamlConfig.(*transformprocessor.Config), gotCfg)

				assert.Equal(t, gotCfg, wantCfg)

			}
		})
	}
}

func TestContainerInsightsJmx(t *testing.T) {
	transl := NewTranslatorWithName(common.PipelineNameContainerInsightsJmx).(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	c := testutil.GetConf(t, "transform_jmx_config.yaml")
	require.NoError(t, c.Unmarshal(&expectedCfg))

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("testdata", "config.json")))
	translatedCfg, err := transl.Translate(conf)
	assert.NoError(t, err)
	actualCfg, ok := translatedCfg.(*transformprocessor.Config)
	assert.True(t, ok)
	assert.Equal(t, len(expectedCfg.MetricStatements), len(actualCfg.MetricStatements))
}

func TestJmxTranslate(t *testing.T) {
	translatorcontext.CurrentContext().SetOs(translatorconfig.OS_TYPE_LINUX)
	transl := NewTranslatorWithName(common.PipelineNameJmx + "/drop").(*translator)
	expectedCfg := transl.factory.CreateDefaultConfig().(*transformprocessor.Config)
	c := testutil.GetConf(t, "transform_jmx_drop_config.yaml")
	require.NoError(t, c.Unmarshal(&expectedCfg))

	conf := confmap.NewFromStringMap(testutil.GetJson(t, filepath.Join("testdata", "config.json")))
	translatedCfg, err := transl.Translate(conf)
	assert.NoError(t, err)
	actualCfg, ok := translatedCfg.(*transformprocessor.Config)
	assert.True(t, ok)

	// sort the statements for consistency
	assert.Len(t, expectedCfg.MetricStatements, 1)
	assert.Len(t, actualCfg.MetricStatements, 1)
	sort.Strings(expectedCfg.MetricStatements[0].Statements)
	sort.Strings(actualCfg.MetricStatements[0].Statements)
}
