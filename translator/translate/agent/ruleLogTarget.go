// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/logger"
)

const (
	lumberjackLogTarget = "logtarget"
)

type LogTarget struct {
}

func (l *LogTarget) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {

	returnKey, returnVal = lumberjackLogTarget, logger.LogTargetLumberjack
	return
}

func init() {
	l := new(LogTarget)
	RegisterRule(lumberjackLogTarget, l)
}
