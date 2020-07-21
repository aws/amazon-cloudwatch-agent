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

const Linux_Default_Log_Dir = "/opt/aws/amazon-cloudwatch-agent/logs/amazon-cloudwatch-agent.log"

type Logfile struct {
}

func (l *Logfile) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("logfile", getDefaultValue(), input)
	return
}

func getDefaultValue() string {
	if context.CurrentContext().RunInContainer() {
		return ""
	}
	targetPlatform := translator.GetTargetPlatform()
	switch targetPlatform {
	case config.OS_TYPE_LINUX:
		return Linux_Default_Log_Dir
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
