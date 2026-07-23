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
				"host_metrics": map[string]interface{}{},
			},
		},
	}
	result := map[string]interface{}{}
	mergeJsonUtil.MergeRuleMap["opentelemetry"].Merge(source, result)

	assert.Contains(t, result, "opentelemetry")
	otel := result["opentelemetry"].(map[string]interface{})
	assert.Contains(t, otel, "collect")
	assert.Contains(t, otel["collect"].(map[string]interface{}), "host_metrics")
}

func TestMergeRule_DatabaseInsights(t *testing.T) {
	// Verify that postgresql and mysql from separate configs merge correctly
	// under database_insights (simulates separate SSM parameters)
	source1 := map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"database_insights": map[string]interface{}{
					"postgresql": []interface{}{
						map[string]interface{}{
							"endpoint":      "localhost:5432",
							"instance_name": "pg-test",
						},
					},
				},
			},
		},
	}
	source2 := map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"database_insights": map[string]interface{}{
					"mysql": []interface{}{
						map[string]interface{}{
							"endpoint":      "localhost:3306",
							"instance_name": "mysql-test",
						},
					},
				},
			},
		},
	}

	// Merge source1 into source2 (source2 acts as result)
	mergeJsonUtil.MergeRuleMap["opentelemetry"].Merge(source1, source2)

	otel := source2["opentelemetry"].(map[string]interface{})
	collect := otel["collect"].(map[string]interface{})
	dbi := collect["database_insights"].(map[string]interface{})

	assert.Contains(t, dbi, "postgresql", "postgresql should be preserved after merge")
	assert.Contains(t, dbi, "mysql", "mysql should be preserved after merge")
}
