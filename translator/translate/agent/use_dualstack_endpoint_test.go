// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/stretchr/testify/assert"
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
				"use_dualstack_endpoint": true,
			},
			expectedKey:    "use_dualstack_endpoint",
			expectedValue:  true,
			expectedGlobal: true,
		},
		"DisableDualStack": {
			input: map[string]interface{}{
				"use_dualstack_endpoint": false,
			},
			expectedKey:    "use_dualstack_endpoint",
			expectedValue:  false,
			expectedGlobal: false,
		},
		"MissingField": {
			input: map[string]interface{}{
				"other_field": "value",
			},
			expectedKey:    "",
			expectedValue:  nil,
			expectedGlobal: false,
		},
		"InvalidFieldTypeString": {
			input: map[string]interface{}{
				"use_dualstack_endpoint": "true",
			},
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false,
		},
		"InvalidFieldTypeInt": {
			input: map[string]interface{}{
				"use_dualstack_endpoint": 1,
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
