// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

const LogGroupNameSectionKey = "log_group_name"

type LogGroupName struct {
}

func (l *LogGroupName) ApplyRule(input interface{}) (string, interface{}) {
	var returnKey string
	var returnVal interface{}
	_, returnVal = translator.DefaultCase(LogGroupNameSectionKey, "", input)
	if returnVal == "" {
		return returnKey, returnVal
	}
	returnKey = "log_group_name"
	returnVal = util.ResolvePlaceholder(returnVal.(string), logs.GlobalLogConfig.MetadataInfo)
	return returnKey, returnVal
}

func init() {
	l := new(LogGroupName)
	RegisterRule(LogGroupNameSectionKey, l)
}
