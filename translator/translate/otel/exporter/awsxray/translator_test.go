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
		input    map[string]any
		want     *confmap.Conf
		wantErr  error
		detector func() (common.Detector, error)
	}{
		"WithMissingKey": {
			input: map[string]any{"logs": map[string]any{}},
			wantErr: &common.MissingKeyError{
				ID:      tt.ID(),
				JsonKey: common.TracesKey,
			},
		},
		"WithDefault": {
			input: map[string]any{"traces": map[string]any{}},
			want: confmap.NewFromStringMap(map[string]any{
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
		},
		"WithCompleteConfig": {
			input: testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "config.yaml")),
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
					"aws.remote.service",
					"aws.remote.operation",
					"HostedIn.K8s.Namespace",
					"K8s.RemoteNamespace",
					"aws.remote.target",
					"HostedIn.Environment",
					"HostedIn.EKS.Cluster",
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
			detector: common.TestEKSDetector,
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
					"aws.remote.service",
					"aws.remote.operation",
					"HostedIn.K8s.Namespace",
					"K8s.RemoteNamespace",
					"aws.remote.target",
					"HostedIn.Environment",
					"HostedIn.K8s.Cluster",
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
			detector: common.TestK8sDetector,
		},
	}
	factory := awsxrayexporter.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			common.NewDetector = testCase.detector
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
