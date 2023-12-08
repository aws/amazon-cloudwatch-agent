// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package translator

import (
	"fmt"
	"strings"
)

// DefaultCase check if current input overrides the default value for the given config entry key.
func DefaultCase(key string, defaultVal, input interface{}) (returnKey string, returnVal interface{}) {
	m, ok := input.(map[string]interface{})
	if !ok {
		// bypassing special formats like:
		//		"procstat": [
		//			{
		//				"pid_file": "/var/run/example1.pid",
		// 				...
		returnKey = ""
		returnVal = ""
	}

	if val, ok := m[key]; ok {
		//The key is in current input instance, use the value in JSON.
		returnVal = val
	} else {
		//The key is not in current input instance, use the default value for the config key
		returnVal = defaultVal
	}
	returnKey = key
	return
}

func DefaultTimeIntervalCase(key string, defaultVal, input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = DefaultCase(key, defaultVal, input)
	// By default json unmarshal will store number as float64
	if floatVal, ok := returnVal.(float64); ok {
		returnVal = fmt.Sprintf("%ds", int(floatVal))
	} else {
		AddErrorMessages(
			fmt.Sprintf("time interval key: %s", key),
			fmt.Sprintf("%s value (%v) in json is not valid for time interval.", key, returnVal))
	}
	return
}

func DefaultIntegralCase(key string, defaultVal, input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = DefaultCase(key, defaultVal, input)
	// By default json unmarshal will store number as float64
	if floatVal, ok := returnVal.(float64); ok {
		returnVal = int(floatVal)
	} else {
		AddErrorMessages(
			fmt.Sprintf("integral key: %s", key),
			fmt.Sprintf("%s value (%v) in json is not valid as an integer.", key, returnVal))
	}
	return
}

func DefaultStringArrayCase(key string, defaultVal, input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = DefaultCase(key, defaultVal, input)
	if arrayVal, ok := returnVal.([]interface{}); ok {
		stringArrayVal := make([]string, len(arrayVal))
		for i, v := range arrayVal {
			stringArrayVal[i] = v.(string)
		}
		returnVal = stringArrayVal
	} else {
		AddErrorMessages(
			fmt.Sprintf("string array key: %s", key),
			fmt.Sprintf("%s value (%v) in json is not valid as an array of strings.", key, returnVal))
	}
	return
}

func DefaultRetentionInDaysCase(key string, defaultVal, input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = DefaultIntegralCase(key, defaultVal, input)
	if intVal, ok := returnVal.(int); ok && IsValidRetentionDays(intVal) {
		returnVal = intVal
	} else {
		returnVal = -1
		AddErrorMessages(
			fmt.Sprintf("Retention in Days key: %s", key),
			fmt.Sprintf("%s value (%v) in is not valid retention in days.", key, returnVal))
	}
	return
}

func DefaultLogGroupClassCase(key string, defaultVal, input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = DefaultCase(key, defaultVal, input)
	if classVal, ok := returnVal.(string); ok && IsValidLogGroupClass(strings.ToUpper(classVal)) {
		//CreateLogGroup API only accepts values STANDARD or INFREQUENT_ACCESS
		returnVal = strings.ToUpper(classVal)
	} else {
		AddInfoMessages(
			fmt.Sprintf("LogGroupClass key: %s", key),
			fmt.Sprintf("%s value (%v) in is not a valid Log Group Class field. Agent will not set the LogGroupClass parameter.", key, returnVal))
		returnVal = ""
	}
	return

}
