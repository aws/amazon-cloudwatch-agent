// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disk

import "github.com/aws/amazon-cloudwatch-agent/translator"

const (
	tagExcludeKey = "tagexclude"
	dropDeviceKey = "drop_device"
)

type DropDevice struct {
}

func (i *DropDevice) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	shouldDropDevice := false //default to be false
	if val, ok := m[dropDeviceKey]; ok {
		delete(m, dropDeviceKey)
		shouldDropDevice = val.(bool)
	}

	if shouldDropDevice {
		returnKey, returnVal = translator.DefaultCase(tagExcludeKey, []string{"device"}, input)
	} else {
		//empty key, value will be ignored when setting the config
		returnKey = ""
		returnVal = ""
	}

	return
}

func init() {
	i := new(DropDevice)
	RegisterRule("drop_device", i)
}
