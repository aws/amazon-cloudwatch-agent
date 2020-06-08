package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type Debug struct {
}

func (d *Debug) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("debug", false, input)
	return
}

func init() {
	d := new(Debug)
	RegisterRule("debug", d)
}
