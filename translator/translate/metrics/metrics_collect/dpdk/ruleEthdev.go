// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package dpdk

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type Ethdev struct {
}

const SectionKey_EthdevExcludeCommands = "ethdev_exclude_commands"

// Querying /ethdev/link_status may take longer to complete and is excluded by
// default, as recommended by the telegraf dpdk plugin documentation.
func (obj *Ethdev) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, val := translator.DefaultCase(SectionKey_EthdevExcludeCommands, []string{"/ethdev/link_status"}, input)
	return "ethdev", map[string]interface{}{"exclude_commands": val}
}

func init() {
	obj := new(Ethdev)
	RegisterRule(SectionKey_EthdevExcludeCommands, obj)
}
