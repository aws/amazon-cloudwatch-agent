// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package dpdk

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type DeviceTypes struct {
}

const SectionKey_DeviceTypes = "device_types"

func (obj *DeviceTypes) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKey_DeviceTypes, []string{"ethdev"}, input)
	return
}

func init() {
	obj := new(DeviceTypes)
	RegisterRule(SectionKey_DeviceTypes, obj)
}
