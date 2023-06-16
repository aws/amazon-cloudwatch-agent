// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package diskio

import "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/util"

type Devices struct {
}

const Devices_Key = "devices"

func (d *Devices) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey = ""
	m := input.(map[string]interface{})

	if _, ok := m[util.Resource_Key]; !ok {
		// TODO: metric aggregation among devices
		return
	}

	if !util.ContainAsterisk(input, util.Resource_Key) {
		returnKey = Devices_Key
		returnVal = m[util.Resource_Key]
	}
	return
}

func init() {
	d := new(Devices)
	RegisterRule(Devices_Key, d)
}
