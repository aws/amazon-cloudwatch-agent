// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsapm

import (
	_ "embed"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/processor/awsapmprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	//go:embed testdata/config_eks.yaml
	validAPMYamlEKS string
	//go:embed testdata/config_generic.yaml
	validAPMYamlGeneric string
	//go:embed testdata/validRulesConfig.json
	validAPMRulesConfig string
	//go:embed testdata/validRulesConfigEKS.yaml
	validAPMRulesYamlEKS string
	//go:embed testdata/validRulesConfigGeneric.yaml
	validAPMRulesYamlGeneric string
	//go:embed testdata/invalidRulesConfig.json
	invalidAPMRulesConfig string
)

func TestTranslate(t *testing.T) {
	var validJsonMap, invalidJsonMap map[string]interface{}
	json.Unmarshal([]byte(validAPMRulesConfig), &validJsonMap)
	json.Unmarshal([]byte(invalidAPMRulesConfig), &invalidJsonMap)

	tt := NewTranslator(WithDataType(component.DataTypeMetrics))
	testCases := map[string]struct {
		input        map[string]interface{}
		want         string
		wantErr      error
		isKubernetes bool
	}{
		//The config for the awsapm processor is https://code.amazon.com/packages/AWSTracingSamplePetClinic/blobs/97ce3c409986ac8ae014de1e3fe71fdb98080f22/--/eks/apm/auto-instrumentation-new.yaml#L20
		//The awsapm processor config does not have a platform field, instead it gets added to resolvers when marshalled
		"WithAPMEnabledEKS": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"apm": map[string]interface{}{},
					},
				}},
			want:         validAPMYamlEKS,
			isKubernetes: true,
		},
		"WithAPMCustomRulesEnabledEKS": {
			input:        validJsonMap,
			want:         validAPMRulesYamlEKS,
			isKubernetes: true,
		},
		"WithAPMEnabledGeneric": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"apm": map[string]interface{}{},
					},
				}},
			want:         validAPMYamlGeneric,
			isKubernetes: false,
		},
		"WithAPMCustomRulesEnabledGeneric": {
			input:        validJsonMap,
			want:         validAPMRulesYamlGeneric,
			isKubernetes: false,
		},
		"WithInvalidAPMCustomRulesEnabled": {
			input:   invalidJsonMap,
			wantErr: errors.New("replace action set, but no replacements defined for service rule"),
		},
	}
	factory := awsapmprocessor.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if testCase.isKubernetes {
				t.Setenv(common.KubernetesEnvVar, "TEST")
			}
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awsapmprocessor.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				yamlConfig, err := common.GetYamlFileToYamlConfig(wantCfg, testCase.want)
				require.NoError(t, err)
				assert.Equal(t, yamlConfig.(*awsapmprocessor.Config), gotCfg)
			}
		})
	}
}
