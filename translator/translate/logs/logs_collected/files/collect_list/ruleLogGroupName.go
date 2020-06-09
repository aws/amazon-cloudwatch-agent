package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

const LogGroupNameSectionKey = "log_group_name"

type LogGroupName struct {
}

func (l *LogGroupName) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase(LogGroupNameSectionKey, "", input)
	if returnVal == "" {
		return
	}
	returnKey = "log_group_name"
	returnVal = util.ResolvePlaceholder(returnVal.(string), logs.GlobalLogConfig.MetadataInfo)
	return
}

func init() {
	l := new(LogGroupName)
	r := []Rule{l}
	RegisterRule(LogGroupNameSectionKey, r)
}
