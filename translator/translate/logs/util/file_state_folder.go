// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/config"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util"
)

const File_State_Folder_Linux = "/opt/aws/amazon-cloudwatch-agent/logs/state"

func GetFileStateFolder() (fileStateFolder string) {
	if translator.GetTargetPlatform() == config.OS_TYPE_WINDOWS {
		fileStateFolder = util.GetWindowsProgramDataPath() + "\\Amazon\\AmazonCloudWatchAgent\\Logs\\state"
	} else {
		fileStateFolder = File_State_Folder_Linux
	}
	return
}
