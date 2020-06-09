package translator

//set sectionKey as routing-tags to all input plugins, all the metrics will go to the output plugins.
//By default input plugin metrics would go through all processor plugins except the plugins defined in "input2Processors"
//input2Processors define processor plugins array for some specific input plugins which don't need to go through all processors

const (
	inputPluginKey     = "inputs"
	processorPluginKey = "processors"
	outputPluginKey    = "outputs"
	tagKey             = "tags"
	tagPassKey         = "tagpass"
	tagExcludeKey      = "tagexclude"
	routingTagKey      = "metricPath"
	linkedCharacter    = "_"
)

// Call this function to set SectionKey as MetricPath TagKey to all input plugins, processor plugins, and the one output plugin
func SetMetricPath(result map[string]interface{}, sectionKey string) {
	if i, ok := result[inputPluginKey]; ok {
		inputs := i.(map[string]interface{})
		for _, v := range inputs {
			inputArray := v.([]interface{})
			for _, inputIntf := range inputArray {
				input := inputIntf.(map[string]interface{})
				if val, ok := input[tagKey]; ok {
					tags := val.(map[string]interface{})
					tags[routingTagKey] = sectionKey
				} else {
					input[tagKey] = map[string]interface{}{routingTagKey: sectionKey}
				}
			}
		}
	}

	if p, ok := result[processorPluginKey]; ok {
		processors := p.(map[string]interface{})
		for _, v := range processors {
			processorArray := v.([]interface{})
			for _, processorIntf := range processorArray {
				processor := processorIntf.(map[string]interface{})
				if val, ok := processor[tagPassKey]; ok {
					tagPass := val.(map[string][]string)
					if tags, ok := tagPass[routingTagKey]; ok {
						tagPass[routingTagKey] = append(tags, sectionKey)
					} else {
						tagPass[routingTagKey] = []string{sectionKey}
					}
				} else {
					processor[tagPassKey] = map[string][]string{routingTagKey: {sectionKey}}
				}
			}
		}
	}

	if o, ok := result[outputPluginKey]; ok {
		outputs := o.(map[string]interface{})
		for _, v := range outputs {
			outputArray := v.([]interface{})
			for _, outputIntf := range outputArray {
				output := outputIntf.(map[string]interface{})
				if val, ok := output[tagPassKey]; ok {
					tagPass := val.(map[string][]string)
					if tags, ok := tagPass[routingTagKey]; ok {
						tagPass[routingTagKey] = append(tags, sectionKey)
					} else {
						tagPass[routingTagKey] = []string{sectionKey}
					}
				} else {
					output[tagPassKey] = map[string][]string{routingTagKey: {sectionKey}}
				}
				if val, ok := output[tagExcludeKey]; ok {
					tagExclude := val.([]string)
					if !contains(tagExclude, routingTagKey) {
						output[tagExcludeKey] = append(tagExclude, routingTagKey)
					}
				} else {
					output[tagExcludeKey] = []string{routingTagKey}
				}
			}
		}
	}
}

// Call this function to override MetricPath for one specific input plugin to go through a subset of processor plugins and then to the one output plugin
func SetMetricPathForOneInput(result map[string]interface{}, sectionKey, inputPlugin string, processorPlugins []string) {
	routingTagVal := sectionKey + linkedCharacter + inputPlugin
	if i, ok := result[inputPluginKey]; ok {
		inputs := i.(map[string]interface{})
		for k, v := range inputs {
			if k != inputPlugin {
				continue
			}
			inputArray := v.([]interface{})
			for _, inputIntf := range inputArray {
				input := inputIntf.(map[string]interface{})
				if val, ok := input[tagKey]; ok {
					tags := val.(map[string]interface{})
					tags[routingTagKey] = routingTagVal
				} else {
					input[tagKey] = map[string]interface{}{routingTagKey: routingTagVal}
				}
			}

		}
	}

	if p, ok := result[processorPluginKey]; ok {
		processors := p.(map[string]interface{})
		for k, v := range processors {
			if !contains(processorPlugins, k) {
				continue
			}
			processorArray := v.([]interface{})
			for _, processorIntf := range processorArray {
				processor := processorIntf.(map[string]interface{})
				if val, ok := processor[tagPassKey]; ok {
					tagPass := val.(map[string][]string)
					if tags, ok := tagPass[routingTagKey]; ok {
						tagPass[routingTagKey] = append(tags, routingTagVal)
					} else {
						tagPass[routingTagKey] = []string{routingTagVal}
					}
				} else {
					processor[tagPassKey] = map[string][]string{routingTagKey: {routingTagVal}}
				}
			}
		}
	}

	if o, ok := result[outputPluginKey]; ok {
		outputs := o.(map[string]interface{})
		for _, v := range outputs {
			outputArray := v.([]interface{})
			for _, outputIntf := range outputArray {
				output := outputIntf.(map[string]interface{})
				if val, ok := output[tagPassKey]; ok {
					tagPass := val.(map[string][]string)
					if tags, ok := tagPass[routingTagKey]; ok {
						tagPass[routingTagKey] = append(tags, routingTagVal)
					} else {
						tagPass[routingTagKey] = []string{routingTagVal}
					}
				} else {
					output[tagPassKey] = map[string][]string{routingTagKey: {routingTagVal}}
				}
				if val, ok := output[tagExcludeKey]; ok {
					tagExclude := val.([]string)
					if !contains(tagExclude, routingTagKey) {
						output[tagExcludeKey] = append(tagExclude, routingTagKey)
					}
				} else {
					output[tagExcludeKey] = []string{routingTagKey}
				}
			}
		}
	}
}

func contains(s []string, target string) bool {
	for _, e := range s {
		if target == e {
			return true
		}
	}
	return false
}
