// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type UseDualStackEndpoint struct {
}

func (r *UseDualStackEndpoint) ApplyRule(input interface{}) (string, interface{}) {
	agentMap, ok := input.(map[string]interface{})
	if !ok {
		return "", translator.ErrorMessages
	}

	dualStackValue, exists := agentMap[UseDualStackEndpointKey]
	if !exists {
		return "", nil
	}

	val, ok := dualStackValue.(bool)
	if !ok {
		return "", translator.ErrorMessages
	}

	Global_Config.UseDualStackEndpoint = val
	return UseDualStackEndpointKey, val
}

func init() {
	RegisterRule(UseDualStackEndpointKey, new(UseDualStackEndpoint))
}
