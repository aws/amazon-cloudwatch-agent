package collected

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type TypesDB struct {
}

const SectionKey_TypesDB = "collectd_typesdb"

func (obj *TypesDB) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKey_TypesDB, []interface{}{"/usr/share/collectd/types.db"}, input)
	return
}

func init() {
	obj := new(TypesDB)
	RegisterRule(SectionKey_TypesDB, obj)
}
