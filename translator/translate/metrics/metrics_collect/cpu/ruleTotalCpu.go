// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cpu

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type TotalCpu struct {
}

func (t *TotalCpu) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("totalcpu", true, input)
	return
}

func init() {
	t := new(TotalCpu)
	RegisterRule("totalcpu", t)
}
