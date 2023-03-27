// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windows

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
)

func AreTwoConfigurationsEqual(config1 NewCwConfig, config2 NewCwConfig) bool {
	// Exception for metrics as the order can be anything
	config1.Metrics = &MetricsEntry{}
	config1.Metrics.MetricsCollect = make(map[string]interface{})
	config2.Metrics = &MetricsEntry{}
	config2.Metrics.MetricsCollect = make(map[string]interface{})

	return reflect.DeepEqual(config1, config2)
}

func ReadNewConfigFromPath(path string) (config NewCwConfig, err error) {
	var file []byte
	if file, err = os.ReadFile(path); err == nil {
		if err = json.Unmarshal(file, &config); err == nil {
			return config, nil
		}
	}
	fmt.Println(err)
	return config, errors.New("failed to parse the expected config")
}

func ReadOldConfigFromPath(path string) (config OldSsmCwConfig, err error) {
	var file []byte
	if file, err = os.ReadFile(path); err == nil {
		if err = json.Unmarshal(file, &config); err == nil {
			return config, nil
		}
	}
	fmt.Println(err)
	return config, errors.New("failed to parse the expected config")
}

func ReadConfigFromPathAsString(path string) (str string, err error) {
	var file []byte
	if file, err = os.ReadFile(path); err == nil {
		return string(file), nil
	}
	fmt.Println(err)
	return str, errors.New("failed to parse the expected config")
}
