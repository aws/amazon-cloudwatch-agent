// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ethtool

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type InterfaceInclude struct {
}

const SectionKey_InterfaceInclude = "interface_include"

func (obj *InterfaceInclude) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKey_InterfaceInclude, []string{"*"}, input)
	return
}

func init() {
	obj := new(InterfaceInclude)
	RegisterRule(SectionKey_InterfaceInclude, obj)
}
