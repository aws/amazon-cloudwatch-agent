package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const BlacklistSectionKey = "blacklist"

type BlackList struct {
}

func (f *BlackList) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase(BlacklistSectionKey, "", input)
	if returnVal == "" {
		return
	}
	returnKey = BlacklistSectionKey
	return
}

func init() {
	b := new(BlackList)
	r := []Rule{b}
	RegisterRule(BlacklistSectionKey, r)
}
