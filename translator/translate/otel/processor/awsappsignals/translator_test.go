// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsappsignals

import (
	_ "embed"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/config"
	translatorConfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	//go:embed testdata/config_eks.yaml
	validAppSignalsYamlEKS string
	//go:embed testdata/config_k8s.yaml
	validAppSignalsYamlK8s string
	//go:embed testdata/config_generic.yaml
	validAppSignalsYamlEC2 string
	//go:embed testdata/config_generic.yaml
	validAppSignalsYamlGeneric string
	//go:embed testdata/validRulesConfig.json
	validAppSignalsRulesConfig string
	//go:embed testdata/validRulesConfigEKS.yaml
	validAppSignalsRulesYamlEKS string
	//go:embed testdata/validRulesConfigGeneric.yaml
	validAppSignalsRulesYamlGeneric string
	//go:embed testdata/invalidRulesConfig.json
	invalidAppSignalsRulesConfig string
)

func TestTranslate(t *testing.T) {
	var validJsonMap, invalidJsonMap map[string]interface{}
	json.Unmarshal([]byte(validAppSignalsRulesConfig), &validJsonMap)
	json.Unmarshal([]byte(invalidAppSignalsRulesConfig), &invalidJsonMap)

	tt := NewTranslator(WithDataType(component.DataTypeMetrics))
	testCases := map[string]struct {
		input          map[string]interface{}
		want           string
		wantErr        error
		isKubernetes   bool
		isEC2          bool
		detector       func() (common.Detector, error)
		isEKSDataStore func() common.IsEKSCache
	}{
		//The config for the awsappsignals processor is https://code.amazon.com/packages/AWSTracingSamplePetClinic/blobs/97ce3c409986ac8ae014de1e3fe71fdb98080f22/--/eks/appsignals/auto-instrumentation-new.yaml#L20
		//The awsappsignals processor config does not have a platform field, instead it gets added to resolvers when marshalled
		"WithAppSignalsEnabledEKS": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{
							"hosted_in": "test",
						},
					},
				}},
			want:           validAppSignalsYamlEKS,
			isKubernetes:   true,
			detector:       common.TestEKSDetector,
			isEKSDataStore: common.TestIsEKSCacheEKS,
		},
		"WithAppSignalsCustomRulesEnabledEKS": {
			input:          validJsonMap,
			want:           validAppSignalsRulesYamlEKS,
			isKubernetes:   true,
			detector:       common.TestEKSDetector,
			isEKSDataStore: common.TestIsEKSCacheEKS,
		},
		"WithAppSignalsEnabledK8S": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{
							"hosted_in": "test",
						},
					},
				}},
			want:           validAppSignalsYamlK8s,
			isKubernetes:   true,
			detector:       common.TestK8sDetector,
			isEKSDataStore: common.TestIsEKSCacheK8s,
		},
		"WithAppSignalsEnabledGeneric": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				}},
			want:         validAppSignalsYamlGeneric,
			isKubernetes: false,
			isEC2:        false,
		},
		"WithAppSignalsCustomRulesEnabledGeneric": {
			input:        validJsonMap,
			want:         validAppSignalsRulesYamlGeneric,
			isKubernetes: false,
		},
		"WithInvalidAppSignalsCustomRulesEnabled": {
			input:   invalidJsonMap,
			wantErr: errors.New("replace action set, but no replacements defined for service rule"),
		},
		"WithAppSignalsEnabledEC2": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{
							"hosted_in": "",
						},
					},
				}},
			want:  validAppSignalsYamlEC2,
			isEC2: true,
		},
	}
	factory := awsappsignals.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if testCase.isKubernetes {
				t.Setenv(common.KubernetesEnvVar, "TEST")
			}
			ctx := context.CurrentContext()
			if testCase.isEC2 {
				ctx.SetMode(translatorConfig.ModeEC2)
			} else {
				ctx.SetMode(translatorConfig.ModeOnPrem)
			}
			common.NewDetector = testCase.detector
			common.IsEKS = testCase.isEKSDataStore
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*config.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				yamlConfig, err := common.GetYamlFileToYamlConfig(wantCfg, testCase.want)
				require.NoError(t, err)
				assert.Equal(t, yamlConfig.(*config.Config), gotCfg)
			}
		})
	}
}
