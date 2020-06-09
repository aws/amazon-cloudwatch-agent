package logs

import "github.com/aws/amazon-cloudwatch-agent/tool/runtime"

type EventConfig struct {
	EventName   string   `event_name`
	EventLevels []string `event_levels`
	EventFormat string   `event_format`
	LogGroup    string   `log_group_name`
	LogStream   string   `log_stream_name`
}

func (config *EventConfig) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	resultMap["event_name"] = config.EventName
	if config.EventLevels != nil && len(config.EventLevels) > 0 {
		resultMap["event_levels"] = config.EventLevels
	}
	if config.EventFormat != "" {
		resultMap["event_format"] = config.EventFormat
	}
	resultMap["log_group_name"] = config.LogGroup
	resultMap["log_stream_name"] = config.LogStream
	return "", resultMap
}
