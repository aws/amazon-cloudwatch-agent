// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/hash"
	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

const (
	Alias_Key                    = "alias"
	Measurement_Key              = "measurement"
	Collect_Interval_Key         = "metrics_collection_interval"
	Collect_Interval_Mapped_Key  = "interval"
	Aggregation_Interval_Key     = "metrics_aggregation_interval"
	Append_Dimensions_Key        = "append_dimensions"
	Append_Dimensions_Mapped_Key = "tags"
	Windows_Object_Name_Key      = "ObjectName"
	Windows_Measurement_Key      = "Measurement"
	Windows_WarnOnMissing_Key    = "WarnOnMissing"
	Windows_Disable_Replacer_Key = "DisableReplacer"
)

// ProcessLinuxCommonConfig is used by both Linux and Darwin.
func ProcessLinuxCommonConfig(input interface{}, pluginName string, path string, result map[string]interface{}) bool {
	isHighResolution := IsHighResolution(agent.Global_Config.Interval)
	inputMap := input.(map[string]interface{})
	// Generate allowlisted metric list, process only if Measurement_Key exist
	if translator.IsValid(inputMap, Measurement_Key, path) {
		// NOTE: the logic here is a bit tricky, even windows uses linux config for metric like procstat, NvidiaGPU.
		os := config.OS_TYPE_LINUX
		if translator.GetTargetPlatform() == config.OS_TYPE_DARWIN {
			os = config.OS_TYPE_DARWIN
		}
		returnKey, returnVal := ApplyMeasurementRule(inputMap[Measurement_Key], pluginName, os, path)
		if returnKey != "" {
			result[returnKey] = returnVal
		} else {
			// No valid metric get generated, stop processing
			return false
		}
	} else {
		return false
	}

	// Set input plugin specific interval
	isHighResolution = setTimeInterval(inputMap, result, isHighResolution, pluginName)

	// Set append_dimensions as tags
	if val, ok := inputMap[Append_Dimensions_Key]; ok {
		result[Append_Dimensions_Mapped_Key] = util.FilterReservedKeys(val)
	}

	// Apply any specific rules for the plugin
	if m, ok := ApplyPluginSpecificRules(pluginName); ok {
		for key, val := range m {
			result[key] = val
		}
	}

	// Add HighResolution tags
	if isHighResolution {
		if result[Append_Dimensions_Mapped_Key] != nil {
			util.AddHighResolutionTag(result[Append_Dimensions_Mapped_Key])
		} else {
			result[Append_Dimensions_Mapped_Key] = map[string]interface{}{util.High_Resolution_Tag_Key: "true"}
		}
	}
	return true
}

// Windows common config returnVal would be three parts:
// 1. interval: Collect_Interval_Mapped_Key
// 2. tags: Append_Dimensions_Mapped_Key
// 3. object config
func ProcessWindowsCommonConfig(input interface{}, pluginName string, path string) (returnVal map[string]interface{}) {
	inputMap := input.(map[string]interface{})
	objectConfig := map[string]interface{}{}
	isHighRsolution := IsHighResolution(agent.Global_Config.Interval)
	returnVal = map[string]interface{}{}

	returnVal[Windows_Disable_Replacer_Key] = true

	// 1. Set input plugin specific interval
	isHighRsolution = setTimeInterval(inputMap, returnVal, isHighRsolution, pluginName)

	// 2. Set append_dimensions as tags
	if val, ok := inputMap[Append_Dimensions_Key]; ok {
		returnVal[Append_Dimensions_Mapped_Key] = val
	}

	// 3. object config
	// Generate allowlisted metric list, process only if Measurement_Key exist
	if translator.IsValid(inputMap, Measurement_Key, path) {
		returnKey, returnVal := ApplyMeasurementRule(inputMap[Measurement_Key], pluginName, config.OS_TYPE_WINDOWS, path)
		if returnKey != "" && len(returnVal) > 0 {
			objectConfig[returnKey] = returnVal
		}
	}

	// 4. Generate a alias name for each windows plugin since every win performance counter plugin will generate
	// a duplicate plugin but with different configuration https://github.com/aws/amazon-cloudwatch-agent/blob/a791b1484fbc0611e515ccbb9bd24bea469cb9fb/translator/translate/metrics/metrics_collect/customizedmetrics/customizedmetric.go#L39-L40
	// and being merged later on if have the same interval,tags, objects https://github.com/aws/amazon-cloudwatch-agent/blob/a791b1484fbc0611e515ccbb9bd24bea469cb9fb/translator/translate/metrics/metrics_collect/customizedmetrics/customizedmetric.go#L58-L86
	returnVal[Alias_Key] = hash.HashName(pluginName)

	// Add common field ObjectName
	objectConfig[Windows_Object_Name_Key] = pluginName

	// Output the message about the missing perf counter metrics
	objectConfig[Windows_WarnOnMissing_Key] = true

	//Measurement behaves like prefix in cloudwatch
	objectConfig[Windows_Measurement_Key] = pluginName

	//instances field is required in windows perf counter config
	if InstanceDisabled(pluginName) {
		objectConfig[Mapped_Instance_Key_Windows] = []string{Disabled_Instance_Val_Windows}
	} else if val, ok := inputMap[Resource_Key]; ok {
		if ContainAsterisk(input, Resource_Key) {
			objectConfig[Mapped_Instance_Key_Windows] = []string{Asterisk_Key}
		} else {
			objectConfig[Mapped_Instance_Key_Windows] = val
		}
	} else {
		// if no instance field specified, we assume the Objects have no instances to collect, use "------" to comply with win_perf_counters.go
		objectConfig[Mapped_Instance_Key_Windows] = []string{Disabled_Instance_Val_Windows}
	}

	// Add HighResolution tags
	if isHighRsolution {
		if returnVal[Append_Dimensions_Mapped_Key] != nil {
			util.AddHighResolutionTag(returnVal[Append_Dimensions_Mapped_Key])
		} else {
			returnVal[Append_Dimensions_Mapped_Key] = map[string]interface{}{util.High_Resolution_Tag_Key: "true"}
		}
	}

	returnVal["object"] = []interface{}{objectConfig}
	return
}

func setTimeInterval(inputMap map[string]interface{}, returnVal map[string]interface{}, isHighRsolution bool, pluginName string) bool {
	if val, ok := inputMap[Collect_Interval_Key]; ok {
		if floatVal, ok := val.(float64); ok {
			val = fmt.Sprintf("%ds", int(floatVal))
			returnVal[Collect_Interval_Mapped_Key] = val
			//Check if this metric is high resolution
			isHighRsolution = IsHighResolution(val.(string))
		} else {
			translator.AddErrorMessages(
				fmt.Sprintf("metrics plugin %s", pluginName),
				fmt.Sprintf("metrics_collection_interval value (%v) in json is not valid for time interval.", val))
		}
	}
	return isHighRsolution
}

func ProcessMetricsCollectionInterval(input interface{}, defaultValue, pluginName string) (returnKey string, returnVal interface{}) {
	if inputMap, ok := input.(map[string]interface{}); ok {
		if val, ok := inputMap[Collect_Interval_Key]; ok {
			if floatVal, ok := val.(float64); ok {
				val = fmt.Sprintf("%ds", int(floatVal))
				return Collect_Interval_Mapped_Key, val
			} else {
				translator.AddErrorMessages(
					fmt.Sprintf("metrics plugin %s", pluginName),
					fmt.Sprintf("metrics_collection_interval value (%v) in json is not valid for time interval.", val))
			}
		}
		if defaultValue != "" {
			return Collect_Interval_Mapped_Key, defaultValue
		}
	}
	return
}

func ProcessMetricsAggregationInterval(input interface{}, defaultValue, pluginName string) (returnKey string, returnVal interface{}) {
	if inputMap, ok := input.(map[string]interface{}); ok {
		if val, ok := inputMap[Aggregation_Interval_Key]; ok {
			if floatVal, ok := val.(float64); ok {
				val = fmt.Sprintf("%ds", int(floatVal))
				if valStr, ok := val.(string); ok && valStr == "0s" {
					// customer specifically disabled the metrics aggregation interval by putting "0"
					return Append_Dimensions_Mapped_Key, map[string]interface{}{util.High_Resolution_Tag_Key: "true"}
				}
				return Append_Dimensions_Mapped_Key, map[string]interface{}{util.Aggregation_Interval_Tag_Key: val}
			} else {
				translator.AddErrorMessages(
					fmt.Sprintf("metrics plugin %s", pluginName),
					fmt.Sprintf("metrics_aggregation_interval value (%v) in json is not valid for time interval.", val))
			}
		}
		if defaultValue != "" {
			return Append_Dimensions_Mapped_Key, map[string]interface{}{util.Aggregation_Interval_Tag_Key: defaultValue}
		}
	}
	return
}

// check if desiredVal exist in inputs list
func ListContains(inputs []string, desiredVal string) bool {
	for _, val := range inputs {
		if val == desiredVal {
			return true
		}
	}
	return false
}
