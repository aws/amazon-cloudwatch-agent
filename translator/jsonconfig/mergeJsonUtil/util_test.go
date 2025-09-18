// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mergeJsonUtil

import (
	"reflect"
	"testing"
)

func TestMergeJmxList(t *testing.T) {
	// Test merging JMX configurations from source into result
	jvm1 := map[string]interface{}{
		"jmx": []interface{}{
			map[string]interface{}{
				"endpoint": "localhost:9999",
				"jvm": map[string]interface{}{
					"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
				},
				"append_dimensions": map[string]interface{}{
					"ProcessGroupName": "MyJVMApp",
				},
			},
		},
	}

	jvm2 := map[string]interface{}{
		"jmx": []interface{}{
			map[string]interface{}{
				"endpoint": "localhost:1234",
				"jvm": map[string]interface{}{
					"measurement": []interface{}{"jvm.classes.loaded", "jvm.memory.heap.committed"},
				},
				"append_dimensions": map[string]interface{}{
					"ProcessGroupName": "MyOtherJVMApp",
				},
			},
		},
	}

	expected := map[string]interface{}{
		"jmx": []interface{}{
			map[string]interface{}{
				"endpoint": "localhost:1234",
				"jvm": map[string]interface{}{
					"measurement": []interface{}{"jvm.classes.loaded", "jvm.memory.heap.committed"},
				},
				"append_dimensions": map[string]interface{}{
					"ProcessGroupName": "MyOtherJVMApp",
				},
			},
			map[string]interface{}{
				"endpoint": "localhost:9999",
				"jvm": map[string]interface{}{
					"measurement": []interface{}{"jvm.memory.heap.used", "jvm.gc.collections.count"},
				},
				"append_dimensions": map[string]interface{}{
					"ProcessGroupName": "MyJVMApp",
				},
			},
		},
	}

	mergeJmxList(jvm1, jvm2, "jmx")

	if !reflect.DeepEqual(resultMap, expected) {
		t.Errorf("mergeJmxList() = %v, want %v", resultMap, expected)
	}
}
