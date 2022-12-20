// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ethtool

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type InterfaceExclude struct {
}

const SectionKey_InterfaceExclude = "interface_exclude"

func (obj *InterfaceExclude) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	key, val := translator.DefaultCase(SectionKey_InterfaceExclude, "", input)
	if val != "" {
		return key, val
	}
	return
}

func init() {
	obj := new(InterfaceExclude)
	RegisterRule(SectionKey_InterfaceExclude, obj)
}
