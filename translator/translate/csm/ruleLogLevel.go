package csm

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/csm"
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type LogLevel struct {
}

func (m *LogLevel) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	key, val := translator.DefaultIntegralCase(csm.LogLevelKey, float64(csm.DefaultLogLevel), input)
	res := map[string]interface{}{}
	res[key] = val

	returnKey = ConfOutputPluginKey
	returnVal = res

	return
}

func init() {
	m := new(LogLevel)
	RegisterRule(csm.LogLevelKey, m)
}
