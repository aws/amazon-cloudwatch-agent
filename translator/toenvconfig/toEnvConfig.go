// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package toenvconfig

import (
	"encoding/json"
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/csm"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

const (
	userAgentKey      = "user_agent"
	debugKey          = "debug"
	awsSdkLogLevelKey = "aws_sdk_log_level"
)

func ToEnvConfig(jsonConfigValue map[string]interface{}) []byte {
	envVars := make(map[string]string)
	// If csm has a configuration section, then also turn on csm for the agent itself
	if _, ok := jsonConfigValue[csm.JSONSectionKey]; ok {
		envVars[envconfig.AWS_CSM_ENABLED] = "TRUE"
	}

	if agentMap, ok := jsonConfigValue[agent.SectionKey].(map[string]interface{}); ok {
		// Set CWAGENT_USER_AGENT to env config if specified by the json config in agent section
		if userAgent, ok := agentMap[userAgentKey].(string); ok {
			envVars[envconfig.CWAGENT_USER_AGENT] = userAgent
		}
		// Set CWAGENT_LOG_LEVEL to DEBUG in env config if present and true in agent section
		if isDebug, ok := agentMap[debugKey].(bool); ok && isDebug {
			envVars[envconfig.CWAGENT_LOG_LEVEL] = "DEBUG"
		}
		if awsSdkLogLevel, ok := agentMap[awsSdkLogLevelKey].(string); ok {
			envVars[envconfig.AWS_SDK_LOG_LEVEL] = awsSdkLogLevel
		}
	}

	proxy := util.GetHttpProxy(context.CurrentContext().Proxy())
	if len(proxy) > 0 {
		envVars[envconfig.HTTP_PROXY] = proxy[commonconfig.HttpProxy]
	}

	proxy = util.GetHttpsProxy(context.CurrentContext().Proxy())
	if len(proxy) > 0 {
		envVars[envconfig.HTTPS_PROXY] = proxy[commonconfig.HttpsProxy]
	}

	proxy = util.GetNoProxy(context.CurrentContext().Proxy())
	if len(proxy) > 0 {
		envVars[envconfig.NO_PROXY] = proxy[commonconfig.NoProxy]
	}

	sslConfig := util.GetSSL(context.CurrentContext().SSL())
	if len(sslConfig) > 0 {
		envVars[envconfig.AWS_CA_BUNDLE] = sslConfig[commonconfig.CABundlePath]
	}

	bytes, err := json.MarshalIndent(envVars, "", "\t")
	if err != nil {
		panic(fmt.Sprintf("Failed to create json map for environment variables. Reason: %s \n", err.Error()))
	}
	return bytes
}
