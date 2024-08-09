// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsapplicationsignals

import (
	_ "embed"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsapplicationsignals/config"
	translatorConfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	//go:embed testdata/config_eks.yaml
	validAppSignalsYamlEKS string
	//go:embed testdata/config_k8s.yaml
	validAppSignalsYamlK8s string
	//go:embed testdata/config_ec2.yaml
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
		kubernetesMode string
		mode           string
	}{
		//The config for the awsapplicationsignals processor is https://code.amazon.com/packages/AWSTracingSamplePetClinic/blobs/97ce3c409986ac8ae014de1e3fe71fdb98080f22/--/eks/appsignals/auto-instrumentation-new.yaml#L20
		//The awsapplicationsignals processor config does not have a platform field, instead it gets added to resolvers when marshalled
		"WithAppSignalsEnabledEKS": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{
							"hosted_in": "test",
						},
					},
				}},
			want:           validAppSignalsYamlEKS,
			isKubernetes:   true,
			kubernetesMode: translatorConfig.ModeEKS,
			mode:           translatorConfig.ModeEC2,
		},
		"WithAppSignalsCustomRulesEnabledEKS": {
			input:          validJsonMap,
			want:           validAppSignalsRulesYamlEKS,
			isKubernetes:   true,
			kubernetesMode: translatorConfig.ModeEKS,
			mode:           translatorConfig.ModeEC2,
		},
		"WithAppSignalsEnabledK8S": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{
							"hosted_in": "test",
						},
					},
				}},
			want:           validAppSignalsYamlK8s,
			isKubernetes:   true,
			kubernetesMode: translatorConfig.ModeK8sEC2,
			mode:           translatorConfig.ModeEC2,
		},
		"WithAppSignalsEnabledGeneric": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{},
					},
				}},
			want:         validAppSignalsYamlGeneric,
			isKubernetes: false,
			mode:         translatorConfig.ModeOnPrem,
		},
		"WithAppSignalsCustomRulesEnabledGeneric": {
			input:        validJsonMap,
			want:         validAppSignalsRulesYamlGeneric,
			isKubernetes: false,
			mode:         translatorConfig.ModeOnPrem,
		},
		"WithInvalidAppSignalsCustomRulesEnabled": {
			input:   invalidJsonMap,
			wantErr: errors.New("replace action set, but no replacements defined for service rule"),
			mode:    translatorConfig.ModeOnPrem,
		},
		"WithAppSignalsEnabledEC2": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"application_signals": map[string]interface{}{
							"hosted_in": "test",
						},
					},
				}},
			want: validAppSignalsYamlEC2,
			mode: translatorConfig.ModeEC2,
		},
		"WithAppSignalsFallbackEnabledK8S": {
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
			kubernetesMode: translatorConfig.ModeK8sEC2,
			mode:           translatorConfig.ModeEC2,
		},
		"WithAppSignalsFallbackEnabledGeneric": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{},
					},
				}},
			want:         validAppSignalsYamlGeneric,
			isKubernetes: false,
			mode:         translatorConfig.ModeOnPrem,
		},
		"WithAppSignalsFallbackEnabledEKS": {
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
			kubernetesMode: translatorConfig.ModeEKS,
			mode:           translatorConfig.ModeEC2,
		},
		"WithAppSignalsFallbackEnabledEC2": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{
							"hosted_in": "test",
						},
					},
				}},
			want: validAppSignalsYamlEC2,
			mode: translatorConfig.ModeEC2,
		},
	}
	factory := awsapplicationsignals.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if testCase.isKubernetes {
				t.Setenv(common.KubernetesEnvVar, "TEST")
			}
			context.CurrentContext().SetKubernetesMode(testCase.kubernetesMode)
			context.CurrentContext().SetMode(testCase.mode)
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
