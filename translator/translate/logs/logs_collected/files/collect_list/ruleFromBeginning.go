// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type FromBeginning struct {
}

func (f *FromBeginning) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("from_beginning", true, input)
	return
}

func init() {
	f := new(FromBeginning)
	r := []Rule{f}
	RegisterRule("from_beginning", r)
}
