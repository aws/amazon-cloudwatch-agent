// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/translator"
)

func TestUseDualStackEndpoint_ApplyRule(t *testing.T) {
	originalUseDualStack := Global_Config.UseDualStackEndpoint
	defer func() {
		Global_Config.UseDualStackEndpoint = originalUseDualStack
	}()

	testCases := map[string]struct {
		input          interface{}
		expectedKey    string
		expectedValue  interface{}
		expectedGlobal bool
	}{
		"EnableDualStack": {
			input: map[string]interface{}{
				UseDualStackEndpointKey: true,
			},
			expectedKey:    UseDualStackEndpointKey,
			expectedValue:  true,
			expectedGlobal: true,
		},
		"DisableDualStack": {
			input: map[string]interface{}{
				UseDualStackEndpointKey: false,
			},
			expectedKey:    UseDualStackEndpointKey,
			expectedValue:  false,
			expectedGlobal: false,
		},
		"InvalidFieldTypeString": {
			input: map[string]interface{}{
				UseDualStackEndpointKey: "true",
			},
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false,
		},
		"InvalidFieldTypeInt": {
			input: map[string]interface{}{
				UseDualStackEndpointKey: 1,
			},
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false,
		},
		"InvalidInputType": {
			input:          "not a map",
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false,
		},
		"InvalidInputNil": {
			input:          nil,
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			Global_Config.UseDualStackEndpoint = false

			rule := &UseDualStackEndpoint{}
			key, value := rule.ApplyRule(testCase.input)

			assert.Equal(t, testCase.expectedKey, key)
			assert.Equal(t, testCase.expectedValue, value)
			assert.Equal(t, testCase.expectedGlobal, Global_Config.UseDualStackEndpoint)
		})
	}
}
