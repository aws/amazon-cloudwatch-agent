// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	translatorConfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/config"
)

const (
	tag_exclude_key         = "tagexclude"
	field_pass_key          = "fieldpass"
	windows_measurement_key = "Counters"
	measurement_name        = "name"
	measurement_category    = "category"
	measurement_rename      = "rename"
	measurement_unit        = "unit"
)

const (
	smi_bin_path             = "bin_path"
	nvidia_smi_plugin_name   = "nvidia_smi"
	Default_Unix_Smi_Path    = "/usr/bin/nvidia-smi"
	Default_Windows_Smi_Path = "C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe"
)

func ApplyMeasurementRule(inputs interface{}, pluginName string, targetOs string, path string) (returnKey string, returnVal []string) {
	inputList := inputs.([]interface{})
	returnKey = ""
	switch targetOs {
	case translatorConfig.OS_TYPE_LINUX, translatorConfig.OS_TYPE_DARWIN:
		returnKey = field_pass_key
	case translatorConfig.OS_TYPE_WINDOWS:
		returnKey = windows_measurement_key
	default:
		// should never happen, only above two osType are supported now
		returnKey = field_pass_key
	}

	returnVal = []string{}
	for _, input := range inputList {
		var inputMetricName interface{}
		if reflect.TypeOf(input).String() == "string" {
			inputMetricName = input
		} else {
			// Then the type of input should be "map[string]interface {}"
			if !translator.IsValid(input, measurement_name, path) {
				continue
			}
			inputMetricName = input.(map[string]interface{})[measurement_name]
		}

		if formatted_metricName := GetValidMetric(targetOs, pluginName, inputMetricName.(string)); formatted_metricName != "" {
			returnVal = append(returnVal, formatted_metricName)
		} else {
			translator.AddErrorMessages(path, "measurement name "+inputMetricName.(string)+" is invalid")
		}
	}

	// If no valid metrics generated, set returnKey to "" for quick check
	if len(returnVal) == 0 {
		returnKey = ""
	}
	return
}

func ApplyMeasurementRuleForMetricDecoration(inputs interface{}, pluginName string, targetOs string) (returnVal []interface{}) {
	inputList := inputs.([]interface{})
	returnVal = []interface{}{}
	pluginName = config.GetRealPluginName(pluginName)
	for _, input := range inputList {
		if reflect.TypeOf(input).String() == "string" {
			continue
		}
		// Then the type of input should be "map[string]interface {}"
		mItemMap := input.(map[string]interface{})
		inputMetricName, ok := mItemMap[measurement_name]
		if !ok {
			// The error message has been captured in ApplyMeasurementRule before, so just skip here
			continue
		}
		if !isDecorationAvail(input.(map[string]interface{})) {
			continue
		}

		formattedMetricName := GetValidMetric(targetOs, pluginName, inputMetricName.(string))

		if formattedMetricName != "" {
			decorationMap := make(map[string]string)
			for k, v := range mItemMap {
				switch k {
				case measurement_name:
					decorationMap[k] = formattedMetricName
				case measurement_rename:
					fallthrough
				case measurement_unit:
					decorationMap[k] = strings.TrimSpace(v.(string))
				default:
					fmt.Printf("Warning, detect unexpected field in measurement: %v", k)
				}
			}
			decorationMap[measurement_category] = pluginName
			returnVal = append(returnVal, decorationMap)
		} else {
			log.Printf("W! Failed to rename metric name: %s, in plugin: %s", inputMetricName, pluginName)
		}
	}
	return
}

func GetValidMetric(targetOs string, pluginName string, metricName string) string {
	var registeredMetrics map[string][]string
	switch targetOs {
	case translatorConfig.OS_TYPE_LINUX:
		registeredMetrics = config.Registered_Metrics_Linux
	case translatorConfig.OS_TYPE_DARWIN:
		registeredMetrics = config.Registered_Metrics_Darwin
	case translatorConfig.OS_TYPE_WINDOWS:
		return metricName
	default:
		log.Panicf("E! Unknown os platform in GetValidMetric: %s", targetOs)
	}
	if val, ok := registeredMetrics[pluginName]; ok {
		formattedMetricName := getFormattedMetricName(metricName, pluginName)
		if ListContains(val, formattedMetricName) {
			return formattedMetricName
		}
	}
	return ""
}

// Do a simple format sanitize
// ex: "cpu_usage_idle" -> "usage_idle"
//     "   cpu_usage_nice " -> "usage_nice"

func getFormattedMetricName(input string, pluginName string) (formattedName string) {
	return strings.TrimPrefix(strings.TrimSpace(input), pluginName+"_")
}

func isDecorationAvail(observationMap map[string]interface{}) bool {
	if _, ok := observationMap[measurement_rename]; ok {
		return true
	}
	if _, ok := observationMap[measurement_unit]; ok {
		return true
	}
	return false
}

// "measurement": [
//
//	{"name": "cpu_usage_idle", "rename": "CPU_USAGE_IDLE", "unit": "unit"},
//	{"name": "cpu_usage_nice", "unit": "unit"},
//	"cpu_usage_guest",
//	"time_active",
//	"usage_active"
//
// ]
func GetMeasurementName(input interface{}) (measurementNames []string) {
	m := input.(map[string]interface{})
	if metricList, ok := m["measurement"]; ok {
		for _, metric := range metricList.([]interface{}) {
			var metricName string
			if strVal, ok := metric.(string); ok {
				metricName = strVal
			} else if mapVal, ok := metric.(map[string]interface{}); ok {
				metricName = mapVal["name"].(string)
			}
			if metricName != "" {
				measurementNames = append(measurementNames, metricName)
			}
		}
	}
	return
}

// ApplyPluginSpecificRules returns a map contains all the rules for tagpass, tagdrop, namepass, namedrop,
// fieldpass, fielddrop, taginclude, tagexclude specifically for certain plugin.
func ApplyPluginSpecificRules(pluginName string) (map[string]interface{}, bool) {
	switch pluginName {
	case nvidia_smi_plugin_name:
		result := map[string]interface{}{tag_exclude_key: GetExcludingTags(pluginName)}
		// if on windows, will look for smi in a windows style path
		if translator.GetTargetPlatform() == translatorConfig.OS_TYPE_WINDOWS {
			// default path for Nvidia_smi.exe is C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe
			// Todo: for windows 10 the path should default to C:\\Windows\\System32\\nvidia-smi.exe will support in the future
			result[smi_bin_path] = Default_Windows_Smi_Path
		}
		return result, true
	default:
		return nil, false
	}
}

func GetExcludingTags(pluginName string) (results []string) {
	return config.TagDenyList[pluginName]
}
