// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//nolint:revive // bypass lint check on new files
package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

var acceptedModes = map[string]interface{}{
	logscommon.LogBackpressureModeDrop: nil,
}

type BackpressureMode struct {
}

func (hr *BackpressureMode) ApplyRule(input any) (string, interface{}) {
	_, returnVal := translator.DefaultCase(logscommon.LogBackpressureModeKey, "", input)
	if returnVal == "" {
		// check for env var as fallback
		returnVal = envconfig.GetLogsBackpressureMode()
		if len(returnVal.(string)) == 0 {
			return "", nil
		}
	}
	returnKey := logscommon.LogBackpressureModeKey
	if _, ok := acceptedModes[returnVal.(string)]; !ok {
		returnVal = ""
	}
	return returnKey, returnVal
}

func init() {
	l := new(BackpressureMode)
	r := []Rule{l}
	RegisterRule(logscommon.LogBackpressureModeKey, r)
}
