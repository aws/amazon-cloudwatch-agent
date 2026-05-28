// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslog

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

func TestCWLExporterTranslator_ID(t *testing.T) {
	tr := newCWLExporterTranslator("syslog_0_default", "/syslog/default", "{hostname}", 0)
	assert.Equal(t, "awscloudwatchlogs/syslog_0_default", tr.ID().String())
}

func TestCWLExporterTranslator_Translate(t *testing.T) {
	testCases := map[string]struct {
		name            string
		logGroupName    string
		logStreamName   string
		retentionInDays int64
		mode            string
		region          string
		roleARN         string
		credentials     map[string]any
		input           map[string]any
		envVars         map[string]string
		wantLogGroup    string
		wantLogStream   string
		wantRetention   int64
		wantRegion      string
		wantRoleARN     string
		wantProfile     string
		wantCredsFile   string
		wantEndpoint    string
		wantLocalMode   bool
		wantRawLog      bool
		wantCertFile    string
	}{
		"BasicConfig": {
			name:          "syslog_0_default",
			logGroupName:  "/syslog/default",
			logStreamName: "{hostname}",
			mode:          config.ModeEC2,
			region:        "us-east-1",
			input:         map[string]any{},
			wantLogGroup:  "/syslog/default",
			wantLogStream: "{hostname}",
			wantRegion:    "us-east-1",
			wantRawLog:    true,
		},
		"WithRetention": {
			name:            "syslog_0_rule_0",
			logGroupName:    "/syslog/auth",
			logStreamName:   "{hostname}",
			retentionInDays: 365,
			mode:            config.ModeEC2,
			region:          "us-west-2",
			input:           map[string]any{},
			wantLogGroup:    "/syslog/auth",
			wantLogStream:   "{hostname}",
			wantRetention:   365,
			wantRegion:      "us-west-2",
			wantRawLog:      true,
		},
		"WithCredentials": {
			name:          "syslog_0_default",
			logGroupName:  "/syslog/default",
			logStreamName: "{hostname}",
			mode:          config.ModeEC2,
			region:        "us-east-1",
			roleARN:       "arn:aws:iam::123456789012:role/GlobalRole",
			credentials: map[string]any{
				"profile":                "my_profile",
				"shared_credential_file": "/home/user/.aws/credentials",
			},
			input:         map[string]any{},
			wantLogGroup:  "/syslog/default",
			wantLogStream: "{hostname}",
			wantRegion:    "us-east-1",
			wantRoleARN:   "arn:aws:iam::123456789012:role/GlobalRole",
			wantProfile:   "my_profile",
			wantCredsFile: "/home/user/.aws/credentials",
			wantRawLog:    true,
		},
		"WithEndpointOverride": {
			name:          "syslog_0_default",
			logGroupName:  "/syslog/default",
			logStreamName: "{hostname}",
			mode:          config.ModeEC2,
			region:        "us-east-1",
			input: map[string]any{
				"logs": map[string]any{
					"endpoint_override": "https://logs.custom.endpoint",
				},
			},
			wantLogGroup:  "/syslog/default",
			wantLogStream: "{hostname}",
			wantRegion:    "us-east-1",
			wantEndpoint:  "https://logs.custom.endpoint",
			wantRawLog:    true,
		},
		"WithLogsRoleARNOverride": {
			name:          "syslog_0_default",
			logGroupName:  "/syslog/default",
			logStreamName: "{hostname}",
			mode:          config.ModeEC2,
			region:        "us-east-1",
			roleARN:       "arn:aws:iam::123456789012:role/GlobalRole",
			input: map[string]any{
				"logs": map[string]any{
					"credentials": map[string]any{
						"role_arn": "arn:aws:iam::123456789012:role/LogsRole",
					},
				},
			},
			wantLogGroup:  "/syslog/default",
			wantLogStream: "{hostname}",
			wantRegion:    "us-east-1",
			wantRoleARN:   "arn:aws:iam::123456789012:role/LogsRole",
			wantRawLog:    true,
		},
		"OnPremMode": {
			name:          "syslog_0_default",
			logGroupName:  "/syslog/default",
			logStreamName: "{hostname}",
			mode:          config.ModeOnPrem,
			region:        "us-east-1",
			input:         map[string]any{},
			wantLogGroup:  "/syslog/default",
			wantLogStream: "{hostname}",
			wantRegion:    "us-east-1",
			wantLocalMode: true,
			wantRawLog:    true,
		},
		"WithCACertBundle": {
			name:          "syslog_0_default",
			logGroupName:  "/syslog/default",
			logStreamName: "{hostname}",
			mode:          config.ModeEC2,
			region:        "us-east-1",
			envVars:       map[string]string{envconfig.AWS_CA_BUNDLE: "/ca/bundle"},
			input:         map[string]any{},
			wantLogGroup:  "/syslog/default",
			wantLogStream: "{hostname}",
			wantRegion:    "us-east-1",
			wantCertFile:  "/ca/bundle",
			wantRawLog:    true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}
			agent.Global_Config.Region = tc.region
			agent.Global_Config.Role_arn = tc.roleARN
			if tc.credentials != nil {
				agent.Global_Config.Credentials = tc.credentials
			} else {
				agent.Global_Config.Credentials = map[string]any{}
			}
			translatorcontext.CurrentContext().SetMode(tc.mode)

			tr := newCWLExporterTranslator(tc.name, tc.logGroupName, tc.logStreamName, tc.retentionInDays)
			conf := confmap.NewFromStringMap(tc.input)
			got, err := tr.Translate(conf)
			require.NoError(t, err)
			require.NotNil(t, got)

			cfg, ok := got.(*awscloudwatchlogsexporter.Config)
			require.True(t, ok)

			assert.Equal(t, tc.wantLogGroup, cfg.LogGroupName)
			assert.Equal(t, tc.wantLogStream, cfg.LogStreamName)
			assert.Equal(t, tc.wantRetention, cfg.LogRetention)
			assert.Equal(t, tc.wantRawLog, cfg.RawLog)
			assert.Equal(t, tc.wantRegion, cfg.Region)
			assert.Equal(t, tc.wantRoleARN, cfg.RoleARN)
			assert.Equal(t, tc.wantLocalMode, cfg.LocalMode)
			assert.Equal(t, tc.wantCertFile, cfg.CertificateFilePath)
			assert.Equal(t, tc.wantEndpoint, cfg.AWSSessionSettings.Endpoint)
			if tc.wantEndpoint != "" {
				assert.Equal(t, tc.wantEndpoint, cfg.Endpoint)
			}
			if tc.wantProfile != "" {
				assert.Equal(t, tc.wantProfile, cfg.Profile)
			}
			if tc.wantCredsFile != "" {
				assert.Equal(t, []string{tc.wantCredsFile}, cfg.SharedCredentialsFile)
			}
		})
	}
}

func TestOTLPExporterTranslator_ID(t *testing.T) {
	tr := newOTLPExporterTranslator("syslog_default")
	assert.Equal(t, "otlphttp/syslog_default", tr.ID().String())
}

func TestOTLPExporterTranslator_Translate(t *testing.T) {
	agent.Global_Config.Region = "us-west-2"
	tr := newOTLPExporterTranslator("syslog_default")
	cfg, err := tr.Translate(confmap.New())
	require.NoError(t, err)

	out := confmap.New()
	require.NoError(t, out.Marshal(cfg))
	assert.Equal(t, "https://logs.us-west-2.amazonaws.com/v1/logs", out.Get("logs_endpoint"))
	assert.Contains(t, fmt.Sprint(out.Get("compression")), "gzip")
	assert.Equal(t, "awscloudwatchlogsprovisioner/syslog_default", out.Get("auth::authenticator"))
}

func TestOTLPExporterTranslator_EndpointOverride(t *testing.T) {
	agent.Global_Config.Region = "us-east-1"
	tr := newOTLPExporterTranslator("syslog_rule_0")
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{"endpoint_override": "https://custom.endpoint.example.com/v1/logs"},
	})
	cfg, err := tr.Translate(conf)
	require.NoError(t, err)

	out := confmap.New()
	require.NoError(t, out.Marshal(cfg))
	assert.Equal(t, "https://custom.endpoint.example.com/v1/logs", out.Get("logs_endpoint"))
}

func TestNewExporterTranslator_Dispatch(t *testing.T) {
	conf := confmap.New()

	t.Run("PLE mode returns CWL exporter", func(t *testing.T) {
		tr := newExporterTranslator("test", "/group", "stream", 7, deliveryModePLE, conf)
		assert.Equal(t, "awscloudwatchlogs/test", tr.ID().String())
	})

	t.Run("OTLP mode returns OTLP exporter", func(t *testing.T) {
		tr := newExporterTranslator("test", "/group", "stream", 7, deliveryModeOTLP, conf)
		assert.Equal(t, "otlphttp/test", tr.ID().String())
	})

	t.Run("empty mode defaults to PLE", func(t *testing.T) {
		tr := newExporterTranslator("test", "/group", "stream", 7, "", conf)
		assert.Equal(t, "awscloudwatchlogs/test", tr.ID().String())
	})
}
