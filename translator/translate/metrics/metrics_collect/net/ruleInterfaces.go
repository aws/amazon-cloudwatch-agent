// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package net

import "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"

type interfaces struct {
}

const Section_Key_Mapped = "interfaces"

func (i *interfaces) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey = ""
	m := input.(map[string]interface{})
	if _, ok := m[util.Resource_Key]; !ok {
		// TODO: metric aggregation among interfaces
		return
	}
	if !util.ContainAsterisk(input, util.Resource_Key) {
		returnKey = Section_Key_Mapped
		returnVal = m[util.Resource_Key]
	}
	return
}

func init() {
	m := new(interfaces)
	RegisterRule(Section_Key_Mapped, m)
}
