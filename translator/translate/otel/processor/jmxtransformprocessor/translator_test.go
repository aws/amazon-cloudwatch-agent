package jmxtransformprocessor

import (
	_ "embed"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
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
		"WithContainerInsights": {
			input:  testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			index:  0,
			wantID: "filter/jmx",
			want:   filepath.Join("testdata", "config.json"),
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
