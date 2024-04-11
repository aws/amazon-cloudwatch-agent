// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsxray

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	t.Setenv(envconfig.AWS_CA_BUNDLE, "/ca/bundle")
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.Role_arn = "global_arn"
	tt := NewTranslator()
	assert.EqualValues(t, "awsxray", tt.ID().String())
	testCases := map[string]struct {
		input          map[string]any
		want           *confmap.Conf
		wantErr        error
		kubernetesMode string
		mode           string
	}{
		"WithMissingKey": {
			input: map[string]any{"logs": map[string]any{}},
			wantErr: &common.MissingKeyError{
				ID:      tt.ID(),
				JsonKey: common.TracesKey,
			},
			mode: config.ModeOnPrem,
		},
		"WithDefault": {
			input: map[string]any{"traces": map[string]any{}},
			want: confmap.NewFromStringMap(map[string]any{
				"certificate_file_path": "/ca/bundle",
				"region":                "us-east-1",
				"local_mode":            "true",
				"role_arn":              "global_arn",
				"imds_retries":          1,
				"telemetry": map[string]any{
					"enabled":          true,
					"include_metadata": true,
				},
				"middleware": "agenthealth/traces",
			}),
			mode: config.ModeOnPrem,
		},
		"WithCompleteConfig": {
			input: testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "config.yaml")),
			mode:  config.ModeOnPrem,
		},
		"WithAppSignalsEnabledEKS": {
			input: map[string]any{
				"traces": map[string]any{
					"traces_collected": map[string]any{
						"app_signals": map[string]any{},
					},
				}},
			want: confmap.NewFromStringMap(map[string]any{
				"indexed_attributes": []string{
					"aws.local.service",
					"aws.local.operation",
					"aws.local.environment",
					"aws.remote.service",
					"aws.remote.operation",
					"aws.remote.environment",
					"aws.remote.resource.identifier",
					"aws.remote.resource.type",
				},
				"certificate_file_path": "/ca/bundle",
				"region":                "us-east-1",
				"role_arn":              "global_arn",
				"imds_retries":          1,
				"telemetry": map[string]any{
					"enabled":          true,
					"include_metadata": true,
				},
				"middleware": "agenthealth/traces",
			}),
			kubernetesMode: config.ModeEKS,
			mode:           config.ModeEC2,
		},
		"WithAppSignalsEnabledK8s": {
			input: map[string]any{
				"traces": map[string]any{
					"traces_collected": map[string]any{
						"app_signals": map[string]any{},
					},
				}},
			want: confmap.NewFromStringMap(map[string]any{
				"indexed_attributes": []string{
					"aws.local.service",
					"aws.local.operation",
					"aws.local.environment",
					"aws.remote.service",
					"aws.remote.operation",
					"aws.remote.environment",
					"aws.remote.resource.identifier",
					"aws.remote.resource.type",
				},
				"certificate_file_path": "/ca/bundle",
				"region":                "us-east-1",
				"role_arn":              "global_arn",
				"imds_retries":          1,
				"telemetry": map[string]any{
					"enabled":          true,
					"include_metadata": true,
				},
				"middleware": "agenthealth/traces",
			}),
			kubernetesMode: config.ModeK8sEC2,
			mode:           config.ModeEC2,
		},
		"WithAppSignalsEnabledEC2": {
			input: map[string]any{
				"traces": map[string]any{
					"traces_collected": map[string]any{
						"app_signals": map[string]any{},
					},
				}},
			want: confmap.NewFromStringMap(map[string]any{
				"indexed_attributes": []string{
					"aws.local.service",
					"aws.local.operation",
					"aws.local.environment",
					"aws.remote.service",
					"aws.remote.operation",
					"aws.remote.environment",
					"aws.remote.resource.identifier",
					"aws.remote.resource.type",
				},
				"certificate_file_path": "/ca/bundle",
				"region":                "us-east-1",
				"role_arn":              "global_arn",
				"imds_retries":          1,
				"telemetry": map[string]any{
					"enabled":          true,
					"include_metadata": true,
				},
				"middleware": "agenthealth/traces",
			}),
			mode: config.ModeEC2,
		},
	}
	factory := awsxrayexporter.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			context.CurrentContext().SetKubernetesMode(testCase.kubernetesMode)
			context.CurrentContext().SetMode(testCase.mode)
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awsxrayexporter.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, component.UnmarshalConfig(testCase.want, wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
