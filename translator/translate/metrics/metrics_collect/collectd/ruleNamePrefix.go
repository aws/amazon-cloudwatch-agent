package collected

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type NamePrefix struct {
}

const SectionKey_NamePrefix = "name_prefix"

func (obj *NamePrefix) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKey_NamePrefix, "collectd_", input)
	return
}

func init() {
	obj := new(NamePrefix)
	RegisterRule(SectionKey_NamePrefix, obj)
}
