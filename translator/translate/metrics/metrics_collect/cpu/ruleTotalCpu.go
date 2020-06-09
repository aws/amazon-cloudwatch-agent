package cpu

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type TotalCpu struct {
}

func (t *TotalCpu) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("totalcpu", true, input)
	return
}

func init() {
	t := new(TotalCpu)
	RegisterRule("totalcpu", t)
}
