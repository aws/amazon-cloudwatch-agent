// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

type LogStreamName struct {
}

func (l *LogStreamName) ApplyRule(input interface{}) (string, interface{}) {
	var returnKey string
	var returnVal interface{}
	key, val := translator.DefaultCase("log_stream_name", "", input)
	if val == "" {
		return returnKey, returnVal
	}
	returnKey = key
	returnVal = util.ResolvePlaceholder(val.(string), logs.GlobalLogConfig.MetadataInfo)
	return returnKey, returnVal
}

func init() {
	l := new(LogStreamName)
	RegisterRule("log_stream_name", l)
}
