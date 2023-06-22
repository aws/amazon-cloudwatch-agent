// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type RoundInterval struct {
}

func (r *RoundInterval) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("round_interval", false, input)
	return
}

func init() {
	r := new(RoundInterval)
	RegisterRule("round_interval", r)
}
