// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package csm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

const (
	overrideMemoryLimit = 10
	overridePort        = 2000
)

func TestCsm_Defaults(t *testing.T) {
	c := new(Csm)
	agent.Global_Config.Region = "us-east-1"

	var input interface{}
	err := json.Unmarshal([]byte(`{"csm":{}}`), &input)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	_, actual := c.ApplyRule(input)
	assert.Equal(t, "", actual)
}
