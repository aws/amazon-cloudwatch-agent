// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/config"
)

const Resource_Key = "resources"
const Asterisk_Key = "*"
const Mapped_Instance_Key_Windows = "Instances"
const Disabled_Instance_Val_Windows = "------"

// If the input map contain instance_key, but the vale is not a string list
// A panic will be thrown
func ContainAsterisk(input interface{}, fieldKey string) bool {
	if val, ok := input.(map[string]interface{}); ok {
		if instanceValue, ok := val[fieldKey]; ok {
			return checkListContainsAsterisk(instanceValue.([]interface{}))
		}
	} else {
		fmt.Printf("Invalid input type, ignore field")
	}
	return false
}

// This method is checking string type only
func checkListContainsAsterisk(inputList []interface{}) bool {
	for _, resource := range inputList {
		if resource.(string) == Asterisk_Key {
			return true
		}
	}
	return false
}

func InstanceDisabled(pluginName string) bool {
	return ListContains(config.Instances_disabled_plugins, pluginName)
}
