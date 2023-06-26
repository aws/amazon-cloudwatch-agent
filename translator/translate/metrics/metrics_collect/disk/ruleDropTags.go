// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disk

import "github.com/aws/private-amazon-cloudwatch-agent-staging/translator"

const (
	tagExcludeKey = "tagexclude"
	dropDeviceKey = "drop_device"
)

var tagExcludeValues = []string{"mode"}

type DropTags struct {
}

func (i *DropTags) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	shouldDropDevice := false //default to be false
	if val, ok := m[dropDeviceKey]; ok {
		delete(m, dropDeviceKey)
		shouldDropDevice = val.(bool)
	}

	if shouldDropDevice {
		returnKey, returnVal = translator.DefaultCase(tagExcludeKey, append([]string{"device"}, tagExcludeValues...), input)
	} else {
		returnKey = tagExcludeKey
		returnVal = tagExcludeValues
	}

	return
}

func init() {
	i := new(DropTags)
	RegisterRule("dropTags", i)
}
