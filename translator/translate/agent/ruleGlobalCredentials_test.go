// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func TestWithAgentConfig(t *testing.T) {
	Global_Config.Credentials = map[string]interface{}{}
	ctx := context.CurrentContext()
	ctx.SetCredentials(map[string]string{})
	c := new(GlobalCreds)
	var input interface{}
	err := json.Unmarshal([]byte(`{ "credentials" : {"access_key":"metric_ak", "secret_key":"metric_sk", "token": "dummy_token", "profile": "dummy_profile", "role_arn": "role_value"}}`), &input)
	if err == nil {
		c.ApplyRule(input)
		assert.Equal(t, "role_value", Global_Config.Role_arn, "Expected to be equal")
	} else {
		panic(err)
	}

}
