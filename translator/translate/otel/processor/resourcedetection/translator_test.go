// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourcedetection

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func TestTranslate(t *testing.T) {
	tt := NewTranslator(WithDataType(component.DataTypeTraces))
	testCases := map[string]struct {
		input          map[string]any
		want           *confmap.Conf
		wantErr        error
		kubernetesMode string
		mode           string
	}{
		"WithAppSignalsEnabled/EKS": {
			input: map[string]any{
				"traces": map[string]any{
					"traces_collected": map[string]any{
						"app_signals": map[string]any{},
					},
				}},
			want:           testutil.GetConf(t, filepath.Join("testdata", "config_eks.yaml")),
			kubernetesMode: translatorconfig.ModeEKS,
			mode:           translatorconfig.ModeEC2,
		},
		"WithAppSignalsEnabled/K8s": {
			input: map[string]any{
				"traces": map[string]any{
					"traces_collected": map[string]any{
						"app_signals": map[string]any{},
					},
				}},
			want:           testutil.GetConf(t, filepath.Join("testdata", "config_generic.yaml")),
			kubernetesMode: translatorconfig.ModeK8sEC2,
			mode:           translatorconfig.ModeEC2,
		},
		"WithAppSignalsEnabled/EC2": {
			input: map[string]any{
				"traces": map[string]any{
					"traces_collected": map[string]any{
						"app_signals": map[string]any{},
					},
				}},
			want: testutil.GetConf(t, filepath.Join("testdata", "config_generic.yaml")),
			mode: translatorconfig.ModeEC2,
		},
	}
	factory := resourcedetectionprocessor.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			context.CurrentContext().SetKubernetesMode(testCase.kubernetesMode)
			context.CurrentContext().SetMode(testCase.mode)
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*resourcedetectionprocessor.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, testCase.want.Unmarshal(wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
