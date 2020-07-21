// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package csm

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/csm"
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type MemoryLimitInMb struct {
}

func (m *MemoryLimitInMb) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	key, val := translator.DefaultIntegralCase(csm.MemoryLimitInMbKey, float64(csm.DefaultMemoryLimitInMb), input)
	res := map[string]interface{}{}
	res[key] = val

	returnKey = ConfOutputPluginKey
	returnVal = res

	return
}

func init() {
	m := new(MemoryLimitInMb)
	RegisterRule(csm.MemoryLimitInMbKey, m)
}
