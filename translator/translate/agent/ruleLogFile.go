// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

const Linux_Darwin_Default_Log_Dir = "/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log"

type Logfile struct {
}

func (l *Logfile) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("logfile", GetDefaultValue(), input)
	context.CurrentContext().SetAgentLogFile(returnVal.(string))
	return
}

func GetDefaultValue() string {
	if context.CurrentContext().RunInContainer() {
		return ""
	}
	if context.CurrentContext().GetAgentLogFile() != "" {
		return context.CurrentContext().GetAgentLogFile()
	}
	targetPlatform := translator.GetTargetPlatform()
	switch targetPlatform {
	case config.OS_TYPE_LINUX, config.OS_TYPE_DARWIN:
		return Linux_Darwin_Default_Log_Dir
	case config.OS_TYPE_WINDOWS:
		return util.GetWindowsProgramDataPath() + "\\Amazon\\AmazonCloudWatchAgent\\Logs\\amazon-cloudwatch-agent.log"
	default:
		panic(fmt.Sprintf("Unsupported platform %v for logRule", targetPlatform))
	}
}

func init() {
	l := new(Logfile)
	RegisterRule("logfile", l)
}
