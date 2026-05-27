// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package toenvconfig

import (
	"maps"
	"os"
	"slices"
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
					agent.UseDualStackEndpointKey: true,
				},
			},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				envconfig.AWS_USE_DUALSTACK_ENDPOINT: "true",
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
					agent.UseDualStackEndpointKey: false,
				},
			},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				envconfig.AWS_USE_DUALSTACK_ENDPOINT: "false",
			},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{})
				context.CurrentContext().SetSSL(map[string]string{})
			},
		},
		{
			name: "combined configuration with dual-stack",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					userAgentKey:                  "custom-agent",
					debugKey:                      true,
					agent.UseDualStackEndpointKey: true,
					awsSdkLogLevelKey:             "INFO",
				},
			},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				envconfig.CWAGENT_USER_AGENT:         "custom-agent",
				envconfig.CWAGENT_LOG_LEVEL:          "DEBUG",
				envconfig.AWS_SDK_LOG_LEVEL:          "INFO",
				envconfig.AWS_USE_DUALSTACK_ENDPOINT: "true",
				envconfig.HTTP_PROXY:                 "http://proxy.test",
				envconfig.AWS_CA_BUNDLE:              "/test/ca-bundle.pem",
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
			name: "invalid dual-stack type string",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					agent.UseDualStackEndpointKey: "true",
				},
			},
			expectedEnv: map[string]string{},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{})
				context.CurrentContext().SetSSL(map[string]string{})
			},
		},
		{
			name: "invalid dual-stack type number",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					agent.UseDualStackEndpointKey: 1,
				},
			},
			expectedEnv: map[string]string{},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{})
				context.CurrentContext().SetSSL(map[string]string{})
			},
		},
		{
			name: "invalid dual-stack type nil",
			input: map[string]interface{}{
				agent.SectionKey: map[string]interface{}{
					agent.UseDualStackEndpointKey: nil,
				},
			},
			expectedEnv: map[string]string{},
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
			assert.Equal(t, tt.expectedEnv, result)
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
			assert.Equal(t, tt.expectedEnv, result)
		})
	}
}

func TestManagedKeys_CoversAllToEnvConfigKeys(t *testing.T) {
	// Set up context to trigger proxy/SSL keys
	context.CurrentContext().SetProxy(map[string]string{
		"http_proxy":  "http://proxy",
		"https_proxy": "https://proxy",
		"no_proxy":    "localhost",
	})
	context.CurrentContext().SetSSL(map[string]string{
		"ca_bundle_path": "/ca.pem",
	})
	defer func() {
		context.CurrentContext().SetProxy(map[string]string{})
		context.CurrentContext().SetSSL(map[string]string{})
	}()

	t.Setenv(envconfig.CWAgentLogsBackpressureMode, "drop")

	// Input that triggers all agent-section keys
	input := map[string]any{
		"agent": map[string]any{
			"user_agent":             "test",
			"debug":                  true,
			"aws_sdk_log_level":      "LogDebug",
			"usage_data":             false,
			"use_dualstack_endpoint": true,
		},
	}

	result := ToEnvConfig(input)
	assert.ElementsMatch(t, ManagedKeys, slices.Collect(maps.Keys(result)),
		"ManagedKeys must exactly match the keys ToEnvConfig can produce")
}
