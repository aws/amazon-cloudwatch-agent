// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscloudwatchlogs

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	legacytranslator "github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	globallogs "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	logsutil "github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

func testMetadata() *logsutil.Metadata {
	return &logsutil.Metadata{
		InstanceID: "some_instance_id",
		Hostname:   "some_hostname",
		PrivateIP:  "some_private_ip",
		AccountID:  "some_account_id",
	}
}

func TestTranslator(t *testing.T) {
	t.Setenv(envconfig.AWS_CA_BUNDLE, "/ca/bundle")
	agent.Global_Config.Region = "us-east-1"
	agent.Global_Config.Role_arn = "global_arn"
	agent.Global_Config.Credentials = map[string]any{
		"profile":                "some_profile",
		"shared_credential_file": "/some/credentials",
	}
	globallogs.GlobalLogConfig.MetadataInfo = logsutil.GetMetadataInfo(testMetadata)
	tt := NewTranslatorWithName(common.PipelineNameEmfLogs)
	require.EqualValues(t, "awscloudwatchlogs/emf_logs", tt.ID().String())
	testCases := map[string]struct {
		env     map[string]string
		input   map[string]any
		mode    string
		want    *confmap.Conf
		wantErr error
	}{
		"WithoutLogStreamName": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"emf": map[string]any{},
					},
				},
			},
			mode: config.ModeEC2,
			want: confmap.NewFromStringMap(map[string]any{
				"certificate_file_path":   "/ca/bundle",
				"emf_only":                true,
				"imds_retries":            1,
				"log_group_name":          "emf/logs/default",
				"log_stream_name":         "some_instance_id",
				"middleware":              "agenthealth/logs",
				"profile":                 "some_profile",
				"raw_log":                 true,
				"region":                  "us-east-1",
				"role_arn":                "global_arn",
				"shared_credentials_file": "/some/credentials",
			}),
		},
		"WithLogStreamName/Basic": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"emf": map[string]any{},
					},
					"log_stream_name": "some random stream",
				},
			},
			mode: config.ModeEC2,
			want: confmap.NewFromStringMap(map[string]any{
				"certificate_file_path":   "/ca/bundle",
				"emf_only":                true,
				"imds_retries":            1,
				"log_group_name":          "emf/logs/default",
				"log_stream_name":         "some random stream",
				"middleware":              "agenthealth/logs",
				"profile":                 "some_profile",
				"raw_log":                 true,
				"region":                  "us-east-1",
				"role_arn":                "global_arn",
				"shared_credentials_file": "/some/credentials",
			}),
		},
		"WithLogStreamName/Placeholder": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"emf": map[string]any{},
					},
					"log_stream_name": "{instance_id}/{hostname}/{unsupported}/stream",
				},
			},
			mode: config.ModeEC2,
			want: confmap.NewFromStringMap(map[string]any{
				"certificate_file_path":   "/ca/bundle",
				"emf_only":                true,
				"imds_retries":            1,
				"log_group_name":          "emf/logs/default",
				"log_stream_name":         "some_instance_id/some_hostname/{unsupported}/stream",
				"middleware":              "agenthealth/logs",
				"profile":                 "some_profile",
				"raw_log":                 true,
				"region":                  "us-east-1",
				"role_arn":                "global_arn",
				"shared_credentials_file": "/some/credentials",
			}),
		},
		"WithCompleteConfig": {
			input: map[string]any{
				"logs": map[string]any{
					"metrics_collected": map[string]any{
						"emf": map[string]any{},
					},
					"log_stream_name": "{hostname}/{ip_address}",
					"credentials": map[string]any{
						"role_arn": "logs_arn",
					},
					"endpoint_override": "https://cloudwatchlogs-endpoint",
				},
			},
			mode: config.ModeOnPrem,
			want: confmap.NewFromStringMap(map[string]any{
				"certificate_file_path":   "/ca/bundle",
				"emf_only":                true,
				"endpoint":                "https://cloudwatchlogs-endpoint",
				"imds_retries":            1,
				"local_mode":              "true",
				"log_group_name":          "emf/logs/default",
				"log_stream_name":         "some_hostname/some_private_ip",
				"middleware":              "agenthealth/logs",
				"profile":                 "some_profile",
				"raw_log":                 true,
				"region":                  "us-east-1",
				"role_arn":                "logs_arn",
				"shared_credentials_file": "/some/credentials",
			}),
		},
	}
	factory := awscloudwatchlogsexporter.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			translatorcontext.CurrentContext().SetMode(testCase.mode)
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			require.Truef(t, legacytranslator.IsTranslateSuccess(), "Error in legacy translation rules: %v", legacytranslator.ErrorMessages)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awscloudwatchlogsexporter.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				require.NoError(t, testCase.want.Unmarshal(wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
