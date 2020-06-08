package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type Quiet struct {
}

func (q *Quiet) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("quiet", false, input)
	return
}

func init() {
	q := new(Quiet)
	RegisterRule("quiet", q)
}
