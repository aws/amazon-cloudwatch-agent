// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otel_aws_cloudwatch_logs

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	legacytranslator "github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	t.Setenv(envconfig.AWS_CA_BUNDLE, "/ca/bundle")
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.Role_arn = "global_arn"
	tt := NewTranslatorWithName(common.PipelineNameEmfLogs)
	require.EqualValues(t, "awscloudwatchlogs/emf_logs", tt.ID().String())
	testCases := map[string]struct {
		env     map[string]string
		input   map[string]any
		want    func() *awscloudwatchlogsexporter.Config
		wantErr error
	}{
		"Basic": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"emf": map[string]any{},
					},
					"log_stream_name": "same random stream",
				},
			},
			want: func() *awscloudwatchlogsexporter.Config {
				cfg := &awscloudwatchlogsexporter.Config{
					LogGroupName:  "emf/logs/default",
					LogStreamName: "same random stream",
					RawLog:        true,
					EmfOnly:       true,
				}
				cfg.AWSSessionSettings.CertificateFilePath = "/ca/bundle"
				cfg.AWSSessionSettings.Region = "us-east-1"
				cfg.AWSSessionSettings.RoleARN = "global_arn"
				return cfg
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			require.Truef(t, legacytranslator.IsTranslateSuccess(), "Error in legacy translation rules: %v", legacytranslator.ErrorMessages)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awscloudwatchlogsexporter.Config)
				require.True(t, ok)
				wantCfg := testCase.want()
				assert.Equal(t, wantCfg.LogGroupName, gotCfg.LogGroupName)
				assert.Equal(t, wantCfg.LogStreamName, gotCfg.LogStreamName)
				assert.Equal(t, wantCfg.RawLog, gotCfg.RawLog)
				assert.Equal(t, wantCfg.EmfOnly, gotCfg.EmfOnly)
				assert.Equal(t, wantCfg.Region, gotCfg.Region)
				assert.Equal(t, wantCfg.RoleARN, gotCfg.RoleARN)
				assert.NotNil(t, gotCfg.MiddlewareID)
				assert.Equal(t, "agenthealth/logs", gotCfg.MiddlewareID.String())
			}
		})
	}
}
