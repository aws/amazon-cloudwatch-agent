// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disk

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

type MountPoints struct {
}

const Section_Key_Mapped = "mount_points"

func (m *MountPoints) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey = ""
	r := input.(map[string]interface{})
	if _, ok := r[util.Resource_Key]; !ok {
		// TODO: metric aggregation among mount points
		return
	}

	if !util.ContainAsterisk(input, util.Resource_Key) {
		returnKey = Section_Key_Mapped
		returnVal = r[util.Resource_Key]
	}
	return
}

func init() {
	m := new(MountPoints)
	RegisterRule("mount_points", m)
}
