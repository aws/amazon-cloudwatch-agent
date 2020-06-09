package collectlist

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

type LogStreamName struct {
}

func (l *LogStreamName) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	key, val := translator.DefaultCase("log_stream_name", "", input)
	if val == "" {
		return
	}
	returnKey = key
	returnVal = util.ResolvePlaceholder(val.(string), logs.GlobalLogConfig.MetadataInfo)
	return
}

func init() {
	l := new(LogStreamName)
	RegisterRule("log_stream_name", l)
}
