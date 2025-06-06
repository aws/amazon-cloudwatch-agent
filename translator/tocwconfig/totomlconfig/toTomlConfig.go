// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package totomlconfig

import (
	"bytes"
	"log"

	"github.com/BurntSushi/toml"
)

func ToTomlConfig(val interface{}) string {
	// Process value to ensure integers in arrays are preserved
	processedVal := processValue(val)

	buf := bytes.Buffer{}
	enc := toml.NewEncoder(&buf)
	err := enc.Encode(processedVal)
	if err != nil {
		log.Panicf("Encode to a valid TOML config fails because of %v", err)
	}
	return buf.String()
}

// Ensures integers in arrays are preserved
func processValue(val interface{}) interface{} {
	switch v := val.(type) {
	case map[string]interface{}:
		for k, value := range v {
			v[k] = processValue(value)
		}
		return v
	case []interface{}:
		for i, value := range v {
			v[i] = processValue(value)
		}
		return v
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for k, value := range v {
			if key, ok := k.(string); ok {
				result[key] = processValue(value)
			}
		}
		return result
	case float64:
		// Convert float64 to int if it's a whole number
		if v == float64(int(v)) {
			return int(v)
		}
		return v
	default:
		return v
	}
}
