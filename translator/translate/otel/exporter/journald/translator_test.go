// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"os"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	globallogs "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

func TestTranslator_ID(t *testing.T) {
	translator := NewTranslator()
	assert.Equal(t, "awscloudwatchlogs", translator.ID().String())

	translatorWithName := NewTranslatorWithName("test_name")
	assert.Equal(t, "awscloudwatchlogs/test_name", translatorWithName.ID().String())
}

func TestTranslator_Translate_BasicConfig(t *testing.T) {
	// Setup
	resetGlobalConfig()
	agent.Global_Config.Region = "us-west-2"
	
	translator := NewTranslator()
	conf := confmap.New()

	// Execute
	result, err := translator.Translate(conf)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, result)

	cfg, ok := result.(*awscloudwatchlogsexporter.Config)
	require.True(t, ok)

	assert.Equal(t, &agenthealth.LogsID, cfg.MiddlewareID)
	assert.Equal(t, "us-west-2", cfg.AWSSessionSettings.Region)
	assert.Equal(t, "", cfg.LogGroupName) // Default empty when no collect config
	assert.Equal(t, "", cfg.LogStreamName) // Default empty when no collect config
}

func TestTranslator_Translate_WithCollectConfig(t *testing.T) {
	// Setup
	resetGlobalConfig()
	agent.Global_Config.Region = "us-east-1"
	
	collectConfig := map[string]interface{}{
		"log_group_name":     "my-journald-logs",
		"log_stream_name":    "{instance_id}",
		"retention_in_days":  float64(7),
	}
	
	translator := NewTranslatorWithConfig("journald_0", collectConfig)
	conf := confmap.New()

	// Execute
	result, err := translator.Translate(conf)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, result)

	cfg, ok := result.(*awscloudwatchlogsexporter.Config)
	require.True(t, ok)

	assert.Equal(t, "my-journald-logs", cfg.LogGroupName)
	assert.Equal(t, "i-UNKNOWN", cfg.LogStreamName) // Placeholder resolved
	assert.Equal(t, int64(7), cfg.LogRetention)
	assert.Equal(t, "us-east-1", cfg.AWSSessionSettings.Region)
}

func TestTranslator_Translate_WithDefaultValues(t *testing.T) {
	// Setup
	resetGlobalConfig()
	agent.Global_Config.Region = "us-west-2"
	
	// Empty collect config should use defaults
	collectConfig := map[string]interface{}{}
	
	translator := NewTranslatorWithConfig("journald_1", collectConfig)
	conf := confmap.New()

	// Execute
	result, err := translator.Translate(conf)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, result)

	cfg, ok := result.(*awscloudwatchlogsexporter.Config)
	require.True(t, ok)

	assert.Equal(t, "journald-logs", cfg.LogGroupName) // Default value
	assert.Equal(t, "{instance_id}", cfg.LogStreamName) // Default {instance_id} not resolved without metadata
}

func TestTranslator_Translate_WithCredentials(t *testing.T) {
	// Setup
	resetGlobalConfig()
	agent.Global_Config.Region = "us-west-2"
	agent.Global_Config.Role_arn = "arn:aws:iam::123456789012:role/CloudWatchAgentServerRole"
	agent.Global_Config.Credentials = map[string]interface{}{
		"profile":           "test-profile",
		"shared_credential_file": "/path/to/credentials",
	}
	
	translator := NewTranslator()
	conf := confmap.New()

	// Execute
	result, err := translator.Translate(conf)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, result)

	cfg, ok := result.(*awscloudwatchlogsexporter.Config)
	require.True(t, ok)

	assert.Equal(t, "arn:aws:iam::123456789012:role/CloudWatchAgentServerRole", cfg.AWSSessionSettings.RoleARN)
	assert.Equal(t, "test-profile", cfg.AWSSessionSettings.Profile)
	assert.Equal(t, []string{"/path/to/credentials"}, cfg.AWSSessionSettings.SharedCredentialsFile)
}

func TestTranslator_Translate_WithEndpointOverride(t *testing.T) {
	// Setup
	resetGlobalConfig()
	agent.Global_Config.Region = "us-west-2"
	
	translator := NewTranslator()
	conf := confmap.New()
	
	// Set endpoint override in config
	confData := map[string]interface{}{
		"logs": map[string]interface{}{
			"endpoint_override": "https://logs-fips.us-west-2.amazonaws.com",
		},
	}
	conf = confmap.NewFromStringMap(confData)

	// Execute
	result, err := translator.Translate(conf)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, result)

	cfg, ok := result.(*awscloudwatchlogsexporter.Config)
	require.True(t, ok)

	assert.Equal(t, "https://logs-fips.us-west-2.amazonaws.com", cfg.Endpoint)
	assert.Equal(t, "https://logs-fips.us-west-2.amazonaws.com", cfg.AWSSessionSettings.Endpoint)
}

func TestTranslator_Translate_WithRoleARNOverride(t *testing.T) {
	// Setup
	resetGlobalConfig()
	agent.Global_Config.Region = "us-west-2"
	agent.Global_Config.Role_arn = "arn:aws:iam::123456789012:role/DefaultRole"
	
	translator := NewTranslator()
	conf := confmap.New()
	
	// Set role ARN override in logs config
	confData := map[string]interface{}{
		"logs": map[string]interface{}{
			"credentials": map[string]interface{}{
				"role_arn": "arn:aws:iam::123456789012:role/LogsRole",
			},
		},
	}
	conf = confmap.NewFromStringMap(confData)

	// Execute
	result, err := translator.Translate(conf)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, result)

	cfg, ok := result.(*awscloudwatchlogsexporter.Config)
	require.True(t, ok)

	// Should use the logs-specific role ARN, not the global one
	assert.Equal(t, "arn:aws:iam::123456789012:role/LogsRole", cfg.AWSSessionSettings.RoleARN)
}

func TestTranslator_Translate_OnPremMode(t *testing.T) {
	// Setup
	resetGlobalConfig()
	agent.Global_Config.Region = "us-west-2"
	
	// Set context to on-premise mode
	ctx := context.CurrentContext()
	ctx.SetMode(config.ModeOnPremise)
	defer ctx.SetMode(config.ModeEC2) // Reset after test
	
	translator := NewTranslator()
	conf := confmap.New()

	// Execute
	result, err := translator.Translate(conf)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, result)

	cfg, ok := result.(*awscloudwatchlogsexporter.Config)
	require.True(t, ok)

	assert.True(t, cfg.AWSSessionSettings.LocalMode)
}

func TestTranslator_Translate_WithCABundle(t *testing.T) {
	// Setup
	resetGlobalConfig()
	agent.Global_Config.Region = "us-west-2"
	
	// Set CA bundle environment variable
	originalCABundle := os.Getenv(envconfig.AWS_CA_BUNDLE)
	defer os.Setenv(envconfig.AWS_CA_BUNDLE, originalCABundle)
	
	testCABundle := "/etc/ssl/certs/ca-bundle.pem"
	os.Setenv(envconfig.AWS_CA_BUNDLE, testCABundle)
	
	translator := NewTranslator()
	conf := confmap.New()

	// Execute
	result, err := translator.Translate(conf)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, result)

	cfg, ok := result.(*awscloudwatchlogsexporter.Config)
	require.True(t, ok)

	assert.Equal(t, testCABundle, cfg.AWSSessionSettings.CertificateFilePath)
}

func TestTranslator_Translate_WithPlaceholderResolution(t *testing.T) {
	// Setup
	resetGlobalConfig()
	agent.Global_Config.Region = "us-west-2"
	
	// Setup global log config with metadata (keys must include braces)
	globallogs.GlobalLogConfig.MetadataInfo = map[string]string{
		"{instance_id}": "i-1234567890abcdef0",
		"{hostname}":    "test-host",
	}
	
	collectConfig := map[string]interface{}{
		"log_group_name":  "logs-{instance_id}",
		"log_stream_name": "{hostname}-stream",
	}
	
	translator := NewTranslatorWithConfig("journald_test", collectConfig)
	conf := confmap.New()

	// Execute
	result, err := translator.Translate(conf)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, result)

	cfg, ok := result.(*awscloudwatchlogsexporter.Config)
	require.True(t, ok)

	assert.Equal(t, "logs-i-1234567890abcdef0", cfg.LogGroupName)
	assert.Equal(t, "test-host-stream", cfg.LogStreamName)
}

// Helper function to reset global config for clean test state
func resetGlobalConfig() {
	agent.Global_Config.Credentials = make(map[string]interface{})
	agent.Global_Config.Region = ""
	agent.Global_Config.Role_arn = ""
	
	globallogs.GlobalLogConfig.MetadataInfo = map[string]string{
		"{instance_id}": "i-UNKNOWN",
		"{hostname}":    "hostname-UNKNOWN",
	}
}