// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
)

func TestMergeRule(t *testing.T) {
	// Verify the opentelemetry merge rule is registered
	_, exists := mergeJsonUtil.MergeRuleMap["opentelemetry"]
	assert.True(t, exists, "opentelemetry merge rule should be registered")

	// Verify merge preserves the opentelemetry section
	source := map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"host_insights": map[string]interface{}{},
			},
		},
	}
	result := map[string]interface{}{}
	mergeJsonUtil.MergeRuleMap["opentelemetry"].Merge(source, result)

	assert.Contains(t, result, "opentelemetry")
	otel := result["opentelemetry"].(map[string]interface{})
	assert.Contains(t, otel, "collect")
}
