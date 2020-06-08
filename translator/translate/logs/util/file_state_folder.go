package util

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
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
