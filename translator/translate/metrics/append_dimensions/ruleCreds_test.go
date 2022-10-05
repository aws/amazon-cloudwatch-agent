// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package append_dimensions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/agent"
)

func TestNoAgentConfi1g(t *testing.T) {
	c := new(Creds)
	var input interface{}
	err := json.Unmarshal([]byte(`{ "cloudwatch_creds" : {"access_key":"metric_ak", "secret_key":"metric_sk", "token": "dummy_token", "profile": "dummy_profile"}}`), &input)
	agent.Global_Config.Credentials = map[string]interface{}{
		"access_key": "global_ak",
		"secret_key": "global_sk",
		"token":      "global_token",
		"profile":    "global_profile",
		"role_arn":   "role_arn",
	}
	if err == nil {
		_, actual := c.ApplyRule(input)
		expected := map[string]interface{}{
			"access_key": "global_ak",
			"secret_key": "global_sk",
			"token":      "global_token",
			"profile":    "global_profile",
		}
		assert.Equal(t, expected, actual, "Expected to be equal")
	} else {
		panic(err)
	}
}
