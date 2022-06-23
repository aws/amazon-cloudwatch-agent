// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package csm

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/csm"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type LogLevel struct {
}

func (m *LogLevel) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	key, val := translator.DefaultIntegralCase(csm.LogLevelKey, float64(csm.DefaultLogLevel), input)
	res := map[string]interface{}{}
	res[key] = val

	returnKey = ConfOutputPluginKey
	returnVal = res

	return
}

func init() {
	m := new(LogLevel)
	RegisterRule(csm.LogLevelKey, m)
}
