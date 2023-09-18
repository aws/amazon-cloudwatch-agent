// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/logger"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

var httpProxy string
var httpsProxy string
var noProxy string

func TestAgentDefaultConfig(t *testing.T) {
	agentDefaultConfig(t, config.OS_TYPE_LINUX)
	agentDefaultConfig(t, config.OS_TYPE_DARWIN)
}

func agentDefaultConfig(t *testing.T, osType string) {
	a := new(Agent)
	translator.SetTargetPlatform(osType)
	var input interface{}
	err := json.Unmarshal([]byte(`{"agent":{"metrics_collection_interval":59, "region": "us-west-2"}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	_, val := a.ApplyRule(input)
	agent := map[string]interface{}{
		"debug":               false,
		"flush_interval":      "1s",
		"flush_jitter":        "0s",
		"hostname":            "",
		"interval":            "59s",
		"logfile":             Linux_Darwin_Default_Log_Dir,
		"metric_batch_size":   1000,
		"metric_buffer_limit": 10000,
		"omit_hostname":       false,
		"precision":           "",
		"quiet":               false,
		"round_interval":      false,
		"collection_jitter":   "0s",
		"logtarget":           "lumberjack",
	}
	assert.Equal(t, agent, val, "Expect to be equal")
}

func TestAgentSpecificConfig(t *testing.T) {
	agentSpecificConfig(t, config.OS_TYPE_LINUX)
	agentSpecificConfig(t, config.OS_TYPE_DARWIN)
}

func agentSpecificConfig(t *testing.T, osType string) {
	translator.SetTargetPlatform(osType)
	a := new(Agent)
	var input interface{}
	err := json.Unmarshal([]byte(`{"agent":{"debug":true, "region": "us-west-2"}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	_, val := a.ApplyRule(input)
	agent := map[string]interface{}{
		"debug":               true,
		"flush_interval":      "1s",
		"flush_jitter":        "0s",
		"hostname":            "",
		"interval":            "60s",
		"logfile":             Linux_Darwin_Default_Log_Dir,
		"logtarget":           logger.LogTargetLumberjack,
		"metric_batch_size":   1000,
		"metric_buffer_limit": 10000,
		"omit_hostname":       false,
		"precision":           "",
		"quiet":               false,
		"round_interval":      false,
		"collection_jitter":   "0s",
	}
	assert.Equal(t, agent, val, "Expect to be equal")
}

func TestNoAgentConfig(t *testing.T) {
	noAgentConfig(t, config.OS_TYPE_LINUX)
	noAgentConfig(t, config.OS_TYPE_DARWIN)
}

func noAgentConfig(t *testing.T, osType string) {
	translator.SetTargetPlatform(osType)
	a := new(Agent)
	var input interface{}
	err := json.Unmarshal([]byte(`{"agent":{"region": "us-west-2"}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	_, val := a.ApplyRule(input)
	agent := map[string]interface{}{
		"debug":               false,
		"flush_interval":      "1s",
		"flush_jitter":        "0s",
		"hostname":            "",
		"interval":            "60s",
		"logfile":             Linux_Darwin_Default_Log_Dir,
		"logtarget":           logger.LogTargetLumberjack,
		"metric_batch_size":   1000,
		"metric_buffer_limit": 10000,
		"omit_hostname":       false,
		"precision":           "",
		"quiet":               false,
		"round_interval":      false,
		"collection_jitter":   "0s",
	}
	assert.Equal(t, agent, val, "Expect to be equal")
}

func TestInternal(t *testing.T) {
	internal(t, config.OS_TYPE_LINUX)
	internal(t, config.OS_TYPE_DARWIN)
}

func internal(t *testing.T, osType string) {
	a := new(Agent)
	translator.SetTargetPlatform(osType)
	var input interface{}
	err := json.Unmarshal([]byte(`{"agent":{"internal": true}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	agent := map[string]interface{}{
		"debug":               false,
		"flush_interval":      "1s",
		"flush_jitter":        "0s",
		"hostname":            "",
		"interval":            "60s",
		"logfile":             Linux_Darwin_Default_Log_Dir,
		"logtarget":           logger.LogTargetLumberjack,
		"metric_batch_size":   1000,
		"metric_buffer_limit": 10000,
		"omit_hostname":       false,
		"precision":           "",
		"quiet":               false,
		"round_interval":      false,
		"collection_jitter":   "0s",
	}

	_, val := a.ApplyRule(input)
	assert.Equal(t, agent, val, "Expect to be equal")
	assert.True(t, Global_Config.Internal)

	err = json.Unmarshal([]byte(`{"agent":{"internal": false}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	_, val = a.ApplyRule(input)
	assert.Equal(t, agent, val, "Expect to be equal")
	assert.False(t, Global_Config.Internal)
}

func saveProxyEnv() {
	httpProxy = os.Getenv("http_proxy")
	httpsProxy = os.Getenv("https_proxy")
	noProxy = os.Getenv("no_proxy")
}

func restoreProxyEnv() {
	os.Setenv("http_proxy", httpProxy)
	os.Setenv("https_proxy", httpsProxy)
	os.Setenv("no_proxy", noProxy)
}
