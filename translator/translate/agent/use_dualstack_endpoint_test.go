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
			input:          true,
			expectedKey:    "use_dualstack_endpoint",
			expectedValue:  true,
			expectedGlobal: true,
		},
		"DisableDualStack": {
			input:          false,
			expectedKey:    "use_dualstack_endpoint",
			expectedValue:  false,
			expectedGlobal: false,
		},
		"InvalidInputString": {
			input:          "true",
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false,
		},
		"InvalidInputInt": {
			input:          1,
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
		"InvalidInputFloat": {
			input:          3.14,
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false,
		},
		"InvalidInputSlice": {
			input:          []string{"true"},
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false,
		},
		"InvalidInputMap": {
			input:          map[string]interface{}{"enabled": true},
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Reset global config before each test
			Global_Config.UseDualStackEndpoint = false

			rule := &UseDualStackEndpoint{}
			key, value := rule.ApplyRule(testCase.input)

			assert.Equal(t, testCase.expectedKey, key)
			assert.Equal(t, testCase.expectedValue, value)
			assert.Equal(t, testCase.expectedGlobal, Global_Config.UseDualStackEndpoint)
		})
	}
}

func TestUseDualStackEndpoint_Registration(t *testing.T) {
	rule, exists := ChildRule["use_dualstack_endpoint"]
	assert.True(t, exists, "use_dualstack_endpoint rule should be registered")
	assert.NotNil(t, rule, "use_dualstack_endpoint rule should not be nil")

	_, ok := rule.(*UseDualStackEndpoint)
	assert.True(t, ok, "registered rule should be of type *UseDualStackEndpoint")
}

func TestUseDualStackEndpoint_GlobalConfigIntegration(t *testing.T) {
	originalUseDualStack := Global_Config.UseDualStackEndpoint
	defer func() {
		Global_Config.UseDualStackEndpoint = originalUseDualStack
	}()

	rule := &UseDualStackEndpoint{}

	Global_Config.UseDualStackEndpoint = false
	key, value := rule.ApplyRule(true)
	assert.Equal(t, "use_dualstack_endpoint", key)
	assert.Equal(t, true, value)
	assert.True(t, Global_Config.UseDualStackEndpoint)

	key, value = rule.ApplyRule(false)
	assert.Equal(t, "use_dualstack_endpoint", key)
	assert.Equal(t, false, value)
	assert.False(t, Global_Config.UseDualStackEndpoint)
}

func TestUseDualStackEndpoint_GlobalConfigPreservation(t *testing.T) {
	originalUseDualStack := Global_Config.UseDualStackEndpoint
	defer func() {
		Global_Config.UseDualStackEndpoint = originalUseDualStack
	}()

	rule := &UseDualStackEndpoint{}

	Global_Config.UseDualStackEndpoint = true
	key, value := rule.ApplyRule("invalid")
	assert.Equal(t, "", key)
	assert.Equal(t, translator.ErrorMessages, value)
	assert.True(t, Global_Config.UseDualStackEndpoint, "Global config should remain unchanged on invalid input")

	Global_Config.UseDualStackEndpoint = false
	key, value = rule.ApplyRule(123)
	assert.Equal(t, "", key)
	assert.Equal(t, translator.ErrorMessages, value)
	assert.False(t, Global_Config.UseDualStackEndpoint, "Global config should remain unchanged on invalid input")
}
