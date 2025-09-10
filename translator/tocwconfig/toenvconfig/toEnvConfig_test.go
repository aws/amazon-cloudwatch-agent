// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package toenvconfig

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
)

func TestToEnvConfig(t *testing.T) {
	tests := []struct {
		name         string
		input        map[string]interface{}
		envVars      map[string]string
		expectedEnv  map[string]string
		contextSetup func()
	}{
		{
			name:        "empty config",
			input:       map[string]interface{}{},
			envVars:     map[string]string{},
			expectedEnv: map[string]string{},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{})
				context.CurrentContext().SetSSL(map[string]string{})
			},
		},
		{
			name: "agent section with all fields",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					userAgentKey:      "custom-agent",
					debugKey:          true,
					awsSdkLogLevelKey: "DEBUG",
					usageDataKey:      false,
				},
			},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				envconfig.CWAGENT_USER_AGENT: "custom-agent",
				envconfig.CWAGENT_LOG_LEVEL:  "DEBUG",
				envconfig.AWS_SDK_LOG_LEVEL:  "DEBUG",
				envconfig.CWAGENT_USAGE_DATA: "FALSE",
			},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{})
				context.CurrentContext().SetSSL(map[string]string{})
			},
		},
		{
			name: "agent section with dual-stack endpoint enabled",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					useDualStackEndpointKey: true,
				},
			},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				"AWS_USE_DUALSTACK_ENDPOINT": "true",
			},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{})
				context.CurrentContext().SetSSL(map[string]string{})
			},
		},
		{
			name: "agent section with dual-stack endpoint disabled",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					useDualStackEndpointKey: false,
				},
			},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				"AWS_USE_DUALSTACK_ENDPOINT": "false",
			},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{})
				context.CurrentContext().SetSSL(map[string]string{})
			},
		},
		{
			name:    "proxy configuration",
			input:   map[string]interface{}{},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				envconfig.HTTP_PROXY:  "http://proxy.example.com",
				envconfig.HTTPS_PROXY: "https://proxy.example.com",
				envconfig.NO_PROXY:    "localhost,127.0.0.1",
			},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{
					"http_proxy":  "http://proxy.example.com",
					"https_proxy": "https://proxy.example.com",
					"no_proxy":    "localhost,127.0.0.1",
				})
				context.CurrentContext().SetSSL(map[string]string{})
			},
		},
		{
			name:    "SSL configuration",
			input:   map[string]interface{}{},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				envconfig.AWS_CA_BUNDLE: "/path/to/ca-bundle.pem",
			},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{})
				context.CurrentContext().SetSSL(map[string]string{
					"ca_bundle_path": "/path/to/ca-bundle.pem",
				})
			},
		},
		{
			name:  "logs section with backpressure drop",
			input: map[string]interface{}{},
			envVars: map[string]string{
				envconfig.CWAgentLogsBackpressureMode: "fd_release",
			},
			expectedEnv: map[string]string{
				envconfig.CWAgentLogsBackpressureMode: "fd_release",
			},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{})
				context.CurrentContext().SetSSL(map[string]string{})
			},
		},
		{
			name: "combined configuration",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					userAgentKey: "custom-agent",
					debugKey:     true,
				},
			},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				envconfig.CWAGENT_USER_AGENT: "custom-agent",
				envconfig.CWAGENT_LOG_LEVEL:  "DEBUG",
				envconfig.HTTP_PROXY:         "http://proxy.test",
				envconfig.AWS_CA_BUNDLE:      "/test/ca-bundle.pem",
			},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{
					"http_proxy": "http://proxy.test",
				})
				context.CurrentContext().SetSSL(map[string]string{
					"ca_bundle_path": "/test/ca-bundle.pem",
				})
			},
		},
		{
			name: "combined configuration with dual-stack",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					userAgentKey:            "custom-agent",
					debugKey:                true,
					useDualStackEndpointKey: true,
					awsSdkLogLevelKey:       "INFO",
				},
			},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				envconfig.CWAGENT_USER_AGENT: "custom-agent",
				envconfig.CWAGENT_LOG_LEVEL:  "DEBUG",
				envconfig.AWS_SDK_LOG_LEVEL:  "INFO",
				"AWS_USE_DUALSTACK_ENDPOINT": "true",
				envconfig.HTTP_PROXY:         "http://proxy.test",
				envconfig.AWS_CA_BUNDLE:      "/test/ca-bundle.pem",
			},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{
					"http_proxy": "http://proxy.test",
				})
				context.CurrentContext().SetSSL(map[string]string{
					"ca_bundle_path": "/test/ca-bundle.pem",
				})
			},
		},
		{
			name: "missing dual-stack configuration defaults to IPv4-only",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					userAgentKey: "test-agent",
				},
			},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				envconfig.CWAGENT_USER_AGENT: "test-agent",
			},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{})
				context.CurrentContext().SetSSL(map[string]string{})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tt.envVars {
					os.Unsetenv(k)
				}
			}()

			tt.contextSetup()
			result := ToEnvConfig(tt.input)
			var actualEnv map[string]string
			err := json.Unmarshal(result, &actualEnv)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedEnv, actualEnv)
		})
	}
}

func TestToEnvConfig_DualStackEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		expectedEnv map[string]string
		description string
	}{
		{
			name: "dual-stack enabled produces correct environment variable",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					useDualStackEndpointKey: true,
				},
			},
			expectedEnv: map[string]string{
				"AWS_USE_DUALSTACK_ENDPOINT": "true",
			},
			description: "When dual-stack is enabled, AWS_USE_DUALSTACK_ENDPOINT should be set to 'true'",
		},
		{
			name: "dual-stack disabled produces correct environment variable",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					useDualStackEndpointKey: false,
				},
			},
			expectedEnv: map[string]string{
				"AWS_USE_DUALSTACK_ENDPOINT": "false",
			},
			description: "When dual-stack is disabled, AWS_USE_DUALSTACK_ENDPOINT should be set to 'false'",
		},
		{
			name: "missing dual-stack configuration defaults to IPv4-only behavior",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					userAgentKey: "test-agent",
				},
			},
			expectedEnv: map[string]string{
				envconfig.CWAGENT_USER_AGENT: "test-agent",
			},
			description: "When dual-stack configuration is missing, no AWS_USE_DUALSTACK_ENDPOINT should be set (defaults to IPv4-only)",
		},
		{
			name: "no agent section defaults to IPv4-only behavior",
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"namespace": "test",
				},
			},
			expectedEnv: map[string]string{},
			description: "When no agent section is present, no AWS_USE_DUALSTACK_ENDPOINT should be set (defaults to IPv4-only)",
		},
		{
			name: "dual-stack with other agent configuration",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					useDualStackEndpointKey: true,
					userAgentKey:            "dual-stack-agent",
					debugKey:                false,
					usageDataKey:            true,
				},
			},
			expectedEnv: map[string]string{
				"AWS_USE_DUALSTACK_ENDPOINT": "true",
				envconfig.CWAGENT_USER_AGENT: "dual-stack-agent",
			},
			description: "Dual-stack configuration should work correctly alongside other agent settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup clean context
			context.CurrentContext().SetProxy(map[string]string{})
			context.CurrentContext().SetSSL(map[string]string{})

			result := ToEnvConfig(tt.input)

			// Verify JSON output format is correct
			var actualEnv map[string]string
			err := json.Unmarshal(result, &actualEnv)
			assert.NoError(t, err, "JSON output should be valid")

			// Verify expected environment variables are set correctly
			assert.Equal(t, tt.expectedEnv, actualEnv, tt.description)

			// Verify JSON is properly formatted (indented)
			var prettyJSON map[string]string
			err = json.Unmarshal(result, &prettyJSON)
			assert.NoError(t, err, "JSON should be parseable")

			if len(tt.expectedEnv) > 0 {
				// Verify the result contains properly formatted JSON
				assert.Contains(t, string(result), "\t", "JSON should be indented with tabs")
			}
		})
	}
}

func TestToEnvConfig_DualStackEndpoint_InvalidTypes(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		expectedEnv map[string]string
		description string
	}{
		{
			name: "invalid dual-stack type string",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					useDualStackEndpointKey: "true",
				},
			},
			expectedEnv: map[string]string{},
			description: "When dual-stack is not a boolean, it should be ignored",
		},
		{
			name: "invalid dual-stack type number",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					useDualStackEndpointKey: 1,
				},
			},
			expectedEnv: map[string]string{},
			description: "When dual-stack is not a boolean, it should be ignored",
		},
		{
			name: "invalid dual-stack type nil",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					useDualStackEndpointKey: nil,
				},
			},
			expectedEnv: map[string]string{},
			description: "When dual-stack is nil, it should be ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup clean context
			context.CurrentContext().SetProxy(map[string]string{})
			context.CurrentContext().SetSSL(map[string]string{})

			result := ToEnvConfig(tt.input)

			// Verify JSON output format is correct
			var actualEnv map[string]string
			err := json.Unmarshal(result, &actualEnv)
			assert.NoError(t, err, "JSON output should be valid")

			// Verify expected environment variables are set correctly
			assert.Equal(t, tt.expectedEnv, actualEnv, tt.description)

			// Specifically verify AWS_USE_DUALSTACK_ENDPOINT is not set
			_, exists := actualEnv["AWS_USE_DUALSTACK_ENDPOINT"]
			assert.False(t, exists, "AWS_USE_DUALSTACK_ENDPOINT should not be set for invalid types")
		})
	}
}

func TestToEnvConfig_TypeAssertions(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		envVars     map[string]string
		expectedEnv map[string]string
	}{
		{
			name: "invalid agent section type",
			input: map[string]interface{}{
				agent.SectionKey: "invalid",
			},
			envVars:     map[string]string{},
			expectedEnv: map[string]string{},
		},
		{
			name: "invalid user_agent type",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					userAgentKey: 123,
				},
			},
			envVars:     map[string]string{},
			expectedEnv: map[string]string{},
		},
		{
			name: "invalid debug type",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					debugKey: "true",
				},
			},
			envVars:     map[string]string{},
			expectedEnv: map[string]string{},
		},
		{
			name: "invalid logs section type",
			input: map[string]interface{}{
				logs.SectionKey: "invalid",
			},
			envVars:     map[string]string{},
			expectedEnv: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tt.envVars {
					os.Unsetenv(k)
				}
			}()

			context.CurrentContext().SetProxy(map[string]string{})
			context.CurrentContext().SetSSL(map[string]string{})
			result := ToEnvConfig(tt.input)
			var actualEnv map[string]string
			err := json.Unmarshal(result, &actualEnv)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedEnv, actualEnv)
		})
	}
}
