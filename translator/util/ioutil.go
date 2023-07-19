// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

// get the json map from the json byte array
func GetJsonMapFromJsonBytes(jsonArray []byte) (map[string]interface{}, error) {
	var c map[string]interface{}

	if err := json.Unmarshal(jsonArray, &c); err != nil {
		return nil, fmt.Errorf("unable to parse json, error: %v", err)
	}
	return c, nil
}

func GetDefaultJsonConfigMap(osType, mode string) (map[string]interface{}, error) {
	defaultJsonConfigByteArray := []byte(config.DefaultJsonConfig(osType, mode))
	return GetJsonMapFromJsonBytes(defaultJsonConfigByteArray)
}

// get the json map from a file
func GetJsonMapFromFile(filename string) (map[string]interface{}, error) {
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return GetJsonMapFromJsonBytes(buf)
}
