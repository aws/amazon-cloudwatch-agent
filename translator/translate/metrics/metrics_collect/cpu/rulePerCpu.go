// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cpu

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/metrics/util"
)

type PerCpu struct {
}

const per_cpu_key = "percpu"

func (t *PerCpu) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey = per_cpu_key
	if util.ContainAsterisk(input, util.Resource_Key) {
		returnVal = true
	} else {
		returnVal = false
	}
	return
}

func init() {
	p := new(PerCpu)
	RegisterRule(per_cpu_key, p)
}
