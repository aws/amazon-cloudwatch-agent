// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/stretchr/testify/assert"
)

func TestUseDualStackEndpoint_ApplyRule(t *testing.T) {
	// Save original state
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
			expectedGlobal: false, // Should remain unchanged
		},
		"InvalidInputInt": {
			input:          1,
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false, // Should remain unchanged
		},
		"InvalidInputNil": {
			input:          nil,
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false, // Should remain unchanged
		},
		"InvalidInputFloat": {
			input:          3.14,
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false, // Should remain unchanged
		},
		"InvalidInputSlice": {
			input:          []string{"true"},
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false, // Should remain unchanged
		},
		"InvalidInputMap": {
			input:          map[string]interface{}{"enabled": true},
			expectedKey:    "",
			expectedValue:  translator.ErrorMessages,
			expectedGlobal: false, // Should remain unchanged
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
	// Test that the rule is properly registered
	rule, exists := ChildRule["use_dualstack_endpoint"]
	assert.True(t, exists, "use_dualstack_endpoint rule should be registered")
	assert.NotNil(t, rule, "use_dualstack_endpoint rule should not be nil")

	// Test that it's the correct type
	_, ok := rule.(*UseDualStackEndpoint)
	assert.True(t, ok, "registered rule should be of type *UseDualStackEndpoint")
}

func TestUseDualStackEndpoint_GlobalConfigIntegration(t *testing.T) {
	// Save original state
	originalUseDualStack := Global_Config.UseDualStackEndpoint
	defer func() {
		Global_Config.UseDualStackEndpoint = originalUseDualStack
	}()

	// Test that Global_Config is properly updated
	rule := &UseDualStackEndpoint{}

	// Enable dual-stack
	Global_Config.UseDualStackEndpoint = false
	key, value := rule.ApplyRule(true)
	assert.Equal(t, "use_dualstack_endpoint", key)
	assert.Equal(t, true, value)
	assert.True(t, Global_Config.UseDualStackEndpoint)

	// Disable dual-stack
	key, value = rule.ApplyRule(false)
	assert.Equal(t, "use_dualstack_endpoint", key)
	assert.Equal(t, false, value)
	assert.False(t, Global_Config.UseDualStackEndpoint)
}

func TestUseDualStackEndpoint_GlobalConfigPreservation(t *testing.T) {
	// Save original state
	originalUseDualStack := Global_Config.UseDualStackEndpoint
	defer func() {
		Global_Config.UseDualStackEndpoint = originalUseDualStack
	}()

	rule := &UseDualStackEndpoint{}

	// Test that invalid input preserves existing global config state
	Global_Config.UseDualStackEndpoint = true
	key, value := rule.ApplyRule("invalid")
	assert.Equal(t, "", key)
	assert.Equal(t, translator.ErrorMessages, value)
	assert.True(t, Global_Config.UseDualStackEndpoint, "Global config should remain unchanged on invalid input")

	// Test with different initial state
	Global_Config.UseDualStackEndpoint = false
	key, value = rule.ApplyRule(123)
	assert.Equal(t, "", key)
	assert.Equal(t, translator.ErrorMessages, value)
	assert.False(t, Global_Config.UseDualStackEndpoint, "Global config should remain unchanged on invalid input")
}
