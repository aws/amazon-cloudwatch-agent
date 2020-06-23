package ethtool

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
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
