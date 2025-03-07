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
			name: "logs section with backpressure drop",
			input: map[string]interface{}{
				logs.SectionKey: map[string]interface{}{
					backpressureDropKey: true,
				},
			},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				envconfig.CWAgentBackpressureDrop: "true",
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
				logs.SectionKey: map[string]interface{}{
					backpressureDropKey: true,
				},
			},
			envVars: map[string]string{},
			expectedEnv: map[string]string{
				envconfig.CWAGENT_USER_AGENT:      "custom-agent",
				envconfig.CWAGENT_LOG_LEVEL:       "DEBUG",
				envconfig.HTTP_PROXY:              "http://proxy.test",
				envconfig.AWS_CA_BUNDLE:           "/test/ca-bundle.pem",
				envconfig.CWAgentBackpressureDrop: "true",
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
			name:  "existing env var without config",
			input: map[string]interface{}{},
			envVars: map[string]string{
				envconfig.CWAgentBackpressureDrop: "true",
			},
			expectedEnv: map[string]string{
				envconfig.CWAgentBackpressureDrop: "true",
			},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{})
				context.CurrentContext().SetSSL(map[string]string{})
			},
		},
		{
			name: "config overrides env var",
			input: map[string]interface{}{
				logs.SectionKey: map[string]interface{}{
					backpressureDropKey: false,
				},
			},
			envVars:     map[string]string{},
			expectedEnv: map[string]string{},
			contextSetup: func() {
				context.CurrentContext().SetProxy(map[string]string{})
				context.CurrentContext().SetSSL(map[string]string{})
			},
		},
		{
			name: "mixed case handling",
			input: map[string]interface{}{
				logs.SectionKey: map[string]interface{}{
					backpressureDropKey: "TRUE",
				},
			},
			envVars:     map[string]string{},
			expectedEnv: map[string]string{},
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
		{
			name: "invalid backpressure_drop type in config",
			input: map[string]interface{}{
				logs.SectionKey: map[string]interface{}{
					backpressureDropKey: 123,
				},
			},
			envVars: map[string]string{
				envconfig.CWAgentBackpressureDrop: "true",
			},
			expectedEnv: map[string]string{
				envconfig.CWAgentBackpressureDrop: "true",
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
