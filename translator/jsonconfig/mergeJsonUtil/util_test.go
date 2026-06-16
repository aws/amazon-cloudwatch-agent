// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mergeJsonUtil // nolint:revive

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeArrayOrObjectConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		sourceMap map[string]interface{}
		resultMap map[string]interface{}
		expected  map[string]interface{}
	}{
		{
			name: "1. Merge two identical JVM objects -> single configuration",
			sourceMap: map[string]interface{}{
				"jmx": map[string]interface{}{
					"endpoint": "localhost:9999",
					"jvm": map[string]interface{}{
						"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
					},
				},
			},
			resultMap: map[string]interface{}{
				"jmx": map[string]interface{}{
					"endpoint": "localhost:9999",
					"jvm": map[string]interface{}{
						"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
					},
				},
			},
			expected: map[string]interface{}{
				"jmx": map[string]interface{}{
					"endpoint": "localhost:9999",
					"jvm": map[string]interface{}{
						"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
					},
				},
			},
		},
		{
			name: "2. Merge two different JVM objects -> array with two objects",
			sourceMap: map[string]interface{}{
				"jmx": map[string]interface{}{
					"endpoint": "localhost:9999",
					"jvm": map[string]interface{}{
						"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
					},
				},
			},
			resultMap: map[string]interface{}{
				"jmx": map[string]interface{}{
					"endpoint": "localhost:1234",
					"jvm": map[string]interface{}{
						"measurement": []interface{}{"jvm.classes.loaded", "jvm.memory.heap.committed"},
					},
				},
			},
			expected: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:1234",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.classes.loaded", "jvm.memory.heap.committed"},
						},
					},
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
						},
					},
				},
			},
		},
		{
			name: "3. Merge JVM array with different object -> array with two objects",
			sourceMap: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
						},
					},
				},
			},
			resultMap: map[string]interface{}{
				"jmx": map[string]interface{}{
					"endpoint": "localhost:1234",
					"jvm": map[string]interface{}{
						"measurement": []interface{}{"jvm.classes.loaded", "jvm.memory.heap.committed"},
					},
				},
			},
			expected: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:1234",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.classes.loaded", "jvm.memory.heap.committed"},
						},
					},
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
						},
					},
				},
			},
		},
		{
			name: "4. Merge two different JVM arrays -> array with two objects",
			sourceMap: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
						},
					},
				},
			},
			resultMap: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:1234",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.classes.loaded", "jvm.memory.heap.committed"},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:1234",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.classes.loaded", "jvm.memory.heap.committed"},
						},
					},
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
						},
					},
				},
			},
		},
		{
			name: "5. Merge two identical JVM arrays -> array with single object",
			sourceMap: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
						},
					},
				},
			},
			resultMap: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
						},
					},
				},
			},
		},
		{
			name: "merge with empty result map",
			sourceMap: map[string]interface{}{
				"jmx": map[string]interface{}{
					"endpoint": "localhost:9999",
					"jvm": map[string]interface{}{
						"measurement": []interface{}{"jvm.memory.heap.used"},
					},
				},
			},
			resultMap: map[string]interface{}{},
			expected: map[string]interface{}{
				"jmx": map[string]interface{}{
					"endpoint": "localhost:9999",
					"jvm": map[string]interface{}{
						"measurement": []interface{}{"jvm.memory.heap.used"},
					},
				},
			},
		},
		{
			name: "merge mixed Kafka and Tomcat configurations",
			sourceMap: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"tomcat": map[string]interface{}{
							"measurement": []interface{}{"tomcat.sessions", "tomcat.errors"},
						},
					},
				},
			},
			resultMap: map[string]interface{}{
				"jmx": map[string]interface{}{
					"endpoint": "localhost:1234",
					"kafka": map[string]interface{}{
						"measurement": []interface{}{"kafka.request.time.avg", "kafka.request.failed"},
					},
				},
			},
			expected: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:1234",
						"kafka": map[string]interface{}{
							"measurement": []interface{}{"kafka.request.time.avg", "kafka.request.failed"},
						},
					},
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"tomcat": map[string]interface{}{
							"measurement": []interface{}{"tomcat.sessions", "tomcat.errors"},
						},
					},
				},
			},
		},
		{
			name: "merge JVM arrays with same endpoint but different measurements -> separate objects",
			sourceMap: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.gc.collections.count"},
						},
					},
				},
			},
			resultMap: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.classes.loaded"},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"jmx": []interface{}{
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.classes.loaded"},
						},
					},
					map[string]interface{}{
						"endpoint": "localhost:9999",
						"jvm": map[string]interface{}{
							"measurement": []interface{}{"jvm.gc.collections.count"},
						},
					},
				},
			},
		},
		{
			name: "merge OTLP objects with different endpoints -> array with both",
			sourceMap: map[string]interface{}{
				"otlp": map[string]interface{}{
					"endpoint": "http://localhost:4318",
					"protocol": "grpc",
				},
			},
			resultMap: map[string]interface{}{
				"otlp": map[string]interface{}{
					"endpoint": "http://localhost:4317",
					"protocol": "grpc",
				},
			},
			expected: map[string]interface{}{
				"otlp": []interface{}{
					map[string]interface{}{
						"endpoint": "http://localhost:4317",
						"protocol": "grpc",
					},
					map[string]interface{}{
						"endpoint": "http://localhost:4318",
						"protocol": "grpc",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Determine the key to test based on the test data
			key := "jmx"
			if _, hasOtlp := tt.sourceMap["otlp"]; hasOtlp {
				key = "otlp"
			} else if _, hasOtlp := tt.resultMap["otlp"]; hasOtlp {
				key = "otlp"
			}

			mergeArrayOrObjectConfiguration(tt.sourceMap, tt.resultMap, key, "/test/path/")
			assert.Equal(t, tt.expected, tt.resultMap)
		})
	}
}

func TestSectionMergeRule(t *testing.T) {
	rule := NewSectionMergeRule("opentelemetry", "/")
	assert.Equal(t, "opentelemetry", rule.SectionKey)
	assert.Equal(t, "/opentelemetry/", rule.Path)
	assert.NotNil(t, rule.MergeMap)
}

func TestSectionMergeRule_Merge(t *testing.T) {
	rule := NewSectionMergeRule("opentelemetry", "/")
	child := NewSectionMergeRule("collect", rule.Path)
	rule.MergeMap[child.SectionKey] = child

	source1 := map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"database_insights": map[string]interface{}{},
			},
		},
	}
	source2 := map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"host_metrics": map[string]interface{}{},
			},
		},
	}

	rule.Merge(source1, source2)

	otel := source2["opentelemetry"].(map[string]interface{})
	collect := otel["collect"].(map[string]interface{})
	assert.Contains(t, collect, "host_metrics")
	assert.Contains(t, collect, "database_insights")
}
