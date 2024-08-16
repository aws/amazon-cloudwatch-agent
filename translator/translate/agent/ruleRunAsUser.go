// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type RunAsUser struct {
}

func (r *RunAsUser) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("run_as_user", nil, input)
	if returnVal == nil {
		returnKey = ""
	}
	return
}

func init() {
	r := new(RunAsUser)
	RegisterRule("run_as_user", r)
}
