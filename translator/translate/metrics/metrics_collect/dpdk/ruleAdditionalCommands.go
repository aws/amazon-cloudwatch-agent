// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package dpdk

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type AdditionalCommands struct {
}

const SectionKey_AdditionalCommands = "additional_commands"

func (obj *AdditionalCommands) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	key, val := translator.DefaultCase(SectionKey_AdditionalCommands, "", input)
	if val != "" {
		return key, val
	}
	return
}

func init() {
	obj := new(AdditionalCommands)
	RegisterRule(SectionKey_AdditionalCommands, obj)
}
