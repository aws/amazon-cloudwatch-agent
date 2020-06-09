package logs

import "github.com/aws/amazon-cloudwatch-agent/tool/runtime"

type Events struct {
	EventConfigs []*EventConfig
}

func (config *Events) ToMap(ctx *runtime.Context) (string, map[string]interface{}) {
	resultMap := make(map[string]interface{})
	collectList := []map[string]interface{}{}
	for i := range config.EventConfigs {
		_, singleEvent := config.EventConfigs[i].ToMap(ctx)
		collectList = append(collectList, singleEvent)
	}
	resultMap["collect_list"] = collectList

	return "windows_events", resultMap
}

func (config *Events) AddWindowsEvent(eventName, logGroupName, logStreamName, eventFormat string, eventLevls []string) {
	if config.EventConfigs == nil {
		config.EventConfigs = []*EventConfig{}
	}
	singleEvent := &EventConfig{
		EventName:   eventName,
		LogGroup:    logGroupName,
		LogStream:   logStreamName,
		EventFormat: eventFormat,
		EventLevels: eventLevls,
	}
	config.EventConfigs = append(config.EventConfigs, singleEvent)

}
