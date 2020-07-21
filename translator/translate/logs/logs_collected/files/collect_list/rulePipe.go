// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type Pipe struct {
}

func (p *Pipe) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("pipe", false, input)
	return
}

func init() {
	p := new(Pipe)
	r := []Rule{p}
	RegisterRule("pipe", r)
}
