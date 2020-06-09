package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type Internal struct {
}

const (
	InternalKey = "internal"
)

// This internal will be provided to the corresponding input and output plugins
// This should be applied before interpreting other component.
func (obj *Internal) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, val := translator.DefaultCase(InternalKey, false, input)
	Global_Config.Internal = val.(bool)
	return
}

func init() {
	obj := new(Internal)
	RegisterRule(InternalKey, obj)
}
