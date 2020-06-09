package logs

import (
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type Collection struct {
	// file based logs
	Files *Files
	// windows events
	WinEvents *Events
}

func (config *Collection) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})

	if config.Files != nil {
		util.AddToMap(ctx, resultMap, config.Files)
	}

	// WinEvents should be nil if the target Os is windows
	if config.WinEvents != nil {
		util.AddToMap(ctx, resultMap, config.WinEvents)
	}

	return "logs_collected", resultMap
}

func (config *Collection) AddWindowsEvent(eventName, logGroupName, logStreamName, eventFormat string, eventLevels []string) {
	if config.WinEvents == nil {
		config.WinEvents = &Events{}
	}
	config.WinEvents.AddWindowsEvent(eventName, logGroupName, logStreamName, eventFormat, eventLevels)
}

func (config *Collection) AddLogFile(filePath, logGroupName, logStreamName string, timestampFormat, timezone, multiLineStartPattern, encoding string) {
	if config.Files == nil {
		config.Files = &Files{}
	}
	config.Files.AddLogFile(filePath, logGroupName, logStreamName, timestampFormat, timezone, multiLineStartPattern, encoding)
}
