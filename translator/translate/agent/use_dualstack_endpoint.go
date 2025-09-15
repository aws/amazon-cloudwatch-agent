// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type UseDualStackEndpoint struct {
}

func (r *UseDualStackEndpoint) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	agentMap, ok := input.(map[string]interface{})
	if !ok {
		returnKey, returnVal = "", translator.ErrorMessages
		return
	}

	dualStackValue, exists := agentMap["use_dualstack_endpoint"]
	if !exists {
		returnKey, returnVal = "", nil
		return
	}

	val, ok := dualStackValue.(bool)
	if !ok {
		returnKey, returnVal = "", translator.ErrorMessages
		return
	}

	Global_Config.UseDualStackEndpoint = val
	returnKey, returnVal = "use_dualstack_endpoint", val
	return
}

func init() {
	RegisterRule("use_dualstack_endpoint", new(UseDualStackEndpoint))
}
