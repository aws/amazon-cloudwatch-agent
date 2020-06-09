package events

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/tool/data"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors"
	"github.com/aws/amazon-cloudwatch-agent/tool/processors/serialization"
	"github.com/aws/amazon-cloudwatch-agent/tool/runtime"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

const (
	VERBOSE     = "VERBOSE"     //5
	INFORMATION = "INFORMATION" //4
	WARNING     = "WARNING"     //3
	ERROR       = "ERROR"       //2
	CRITICAL    = "CRITICAL"    //1

	WindowsEventLog = "Windows event log"

	EventFormatXMLDescription       = "XML: XML format in Windows Event Viewer"
	EventFormatPlainTextDescription = "Plain Text: Legacy CloudWatch Windows Agent (SSM Plugin) Format"

	EventFormatXML       = "xml"
	EventFormatPlainText = "text"
)

var Processor processors.Processor = &processor{}

type processor struct{}

func (p *processor) Process(ctx *runtime.Context, config *data.Config) {
	monitorEvents(ctx, config)
}

func (p *processor) NextProcessor(ctx *runtime.Context, config *data.Config) interface{} {
	return serialization.Processor
}

func monitorEvents(ctx *runtime.Context, config *data.Config) {
	yes := util.Yes(fmt.Sprintf("Do you want to monitor any %s?", WindowsEventLog))
	if !yes {
		return
	}

	eventFormatDefaultOption := 1
	for {
		logsConf := config.LogsConf()
		eventName := util.AskWithDefault(fmt.Sprintf("%s name:", WindowsEventLog), "System")

		availableEventLevels := []string{VERBOSE, INFORMATION, WARNING, ERROR, CRITICAL}
		eventLevels := []string{}
		for _, eventLevel := range availableEventLevels {
			yes = util.Yes(
				fmt.Sprintf("Do you want to monitor %s level events for %s %s ?",
					eventLevel,
					WindowsEventLog,
					eventName))
			if yes {
				eventLevels = append(eventLevels, eventLevel)
			}
		}

		logGroupName := util.AskWithDefault("Log group name:", eventName)

		logStreamNameHint := "{instance_id}"
		if ctx.IsOnPrem {
			logStreamNameHint = "{hostname}"
		}

		logStreamName := util.AskWithDefault("Log stream name:", logStreamNameHint)

		eventFormat := util.Choice("In which format do you want to store windows event to CloudWatch Logs?", eventFormatDefaultOption, []string{EventFormatXMLDescription, EventFormatPlainTextDescription})
		if eventFormat == EventFormatXMLDescription {
			eventFormat = EventFormatXML
			eventFormatDefaultOption = 1
		} else {
			eventFormat = EventFormatPlainText
			eventFormatDefaultOption = 2
		}

		logsConf.AddWindowsEvent(eventName, logGroupName, logStreamName, eventFormat, eventLevels)

		yes = util.Yes(fmt.Sprintf("Do you want to specify any additional %s to monitor?", WindowsEventLog))
		if !yes {
			return
		}
	}
}
