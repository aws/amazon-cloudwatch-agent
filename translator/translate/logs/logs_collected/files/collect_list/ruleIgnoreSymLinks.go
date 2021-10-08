// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type IgnoreSymLinks struct {
}

func (f *IgnoreSymLinks) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("ignore_symlinks", false, input)
	return
}

func init() {
	f := new(IgnoreSymLinks)
	r := []Rule{f}
	RegisterRule("ignore_symlinks", r)
}
