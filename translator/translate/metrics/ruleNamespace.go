package metrics

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type Namespace struct {
}

func (n *Namespace) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	res := map[string]interface{}{}
	key, val := translator.DefaultCase("namespace", "CWAgent", input)
	res[key] = val
	returnKey = "outputs"
	returnVal = res
	return
}

func init() {
	n := new(Namespace)
	RegisterRule("namespace", n)
}
