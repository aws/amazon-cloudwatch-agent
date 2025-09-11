// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"log"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type UseDualStackEndpoint struct {
}

func (r *UseDualStackEndpoint) ApplyRule(input interface{}) (string, interface{}) {
	log.Printf("[DEBUG] UseDualStackEndpoint - ApplyRule called with input: %v (type: %T)", input, input)

	// Extract the agent map from input
	agentMap, ok := input.(map[string]interface{})
	if !ok {
		log.Printf("[DEBUG] UseDualStackEndpoint - Input is not a map, returning error")
		return "", translator.ErrorMessages
	}

	// Extract the use_dualstack_endpoint field from the agent map
	dualStackValue, exists := agentMap["use_dualstack_endpoint"]
	if !exists {
		log.Printf("[DEBUG] UseDualStackEndpoint - use_dualstack_endpoint field not found in agent config")
		return "", nil
	}

	val, ok := dualStackValue.(bool)
	if !ok {
		log.Printf("[DEBUG] UseDualStackEndpoint - use_dualstack_endpoint value is not a boolean: %v (type: %T)", dualStackValue, dualStackValue)
		return "", translator.ErrorMessages
	}

	log.Printf("[DEBUG] UseDualStackEndpoint - Setting Global_Config.UseDualStackEndpoint = %t", val)
	Global_Config.UseDualStackEndpoint = val
	log.Printf("[DEBUG] UseDualStackEndpoint - Global_Config.UseDualStackEndpoint is now: %t", Global_Config.UseDualStackEndpoint)
	return "use_dualstack_endpoint", val
}

func init() {
	RegisterRule("use_dualstack_endpoint", new(UseDualStackEndpoint))
}
