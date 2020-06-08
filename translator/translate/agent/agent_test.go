package agent

import (
	"encoding/json"
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/logger"

	"github.com/aws/amazon-cloudwatch-agent/translator"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"

	"os"

	"github.com/stretchr/testify/assert"
)

var httpProxy string
var httpsProxy string
var noProxy string

func TestAgentDefaultConfig(t *testing.T) {
	a := new(Agent)
	translator.SetTargetPlatform(config.OS_TYPE_LINUX)
	var input interface{}
	e := json.Unmarshal([]byte(`{"agent":{"metrics_collection_interval":59, "region": "us-west-2"}}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := a.ApplyRule(input)
	agent := map[string]interface{}{
		"debug":               false,
		"flush_interval":      "1s",
		"flush_jitter":        "0s",
		"hostname":            "",
		"interval":            "59s",
		"logfile":             Linux_Default_Log_Dir,
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
	a := new(Agent)
	translator.SetTargetPlatform(config.OS_TYPE_LINUX)
	var input interface{}
	e := json.Unmarshal([]byte(`{"agent":{"debug":true, "region": "us-west-2"}}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}
	_, val := a.ApplyRule(input)
	agent := map[string]interface{}{
		"debug":               true,
		"flush_interval":      "1s",
		"flush_jitter":        "0s",
		"hostname":            "",
		"interval":            "60s",
		"logfile":             Linux_Default_Log_Dir,
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
	a := new(Agent)
	translator.SetTargetPlatform(config.OS_TYPE_LINUX)
	var input interface{}
	e := json.Unmarshal([]byte(`{"agent":{"region": "us-west-2"}}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}

	_, val := a.ApplyRule(input)
	agent := map[string]interface{}{
		"debug":               false,
		"flush_interval":      "1s",
		"flush_jitter":        "0s",
		"hostname":            "",
		"interval":            "60s",
		"logfile":             Linux_Default_Log_Dir,
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
	a := new(Agent)
	translator.SetTargetPlatform(config.OS_TYPE_LINUX)
	var input interface{}
	e := json.Unmarshal([]byte(`{"agent":{"internal": true}}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
	}

	agent := map[string]interface{}{
		"debug":               false,
		"flush_interval":      "1s",
		"flush_jitter":        "0s",
		"hostname":            "",
		"interval":            "60s",
		"logfile":             Linux_Default_Log_Dir,
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

	e = json.Unmarshal([]byte(`{"agent":{"internal": false}}`), &input)
	if e != nil {
		assert.Fail(t, e.Error())
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
