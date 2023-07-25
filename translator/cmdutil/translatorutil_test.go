// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
)

func TestTranslateJsonMapToEnvConfigFile(t *testing.T) {
	jsonConfigValue := map[string]interface{}{
		"agent": map[string]interface{}{
			"user_agent":        "cwagent",
			"debug":             true,
			"aws_sdk_log_level": "loglevel",
		},
	}
	envConfigPath := path.Join(t.TempDir(), "env-config.json")
	expectedFile := "testdata/env-config.json"

	TranslateJsonMapToEnvConfigFile(jsonConfigValue, envConfigPath)

	var actualJson map[string]interface{}
	var expectedJson map[string]interface{}
	actual, _ := os.ReadFile(envConfigPath)
	expected, _ := os.ReadFile(expectedFile)
	json.Unmarshal(actual, actualJson)
	json.Unmarshal(expected, expectedJson)

	assert.Equal(t, expectedJson[envconfig.CWAGENT_USER_AGENT], actualJson[envconfig.CWAGENT_USER_AGENT])
	assert.Equal(t, expectedJson[envconfig.CWAGENT_LOG_LEVEL], actualJson[envconfig.CWAGENT_LOG_LEVEL])
	assert.Equal(t, expectedJson[envconfig.AWS_SDK_LOG_LEVEL], actualJson[envconfig.AWS_SDK_LOG_LEVEL])
}
