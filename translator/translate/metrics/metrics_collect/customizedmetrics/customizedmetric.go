// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package customizedmetrics

import (
	"reflect"
	"sort"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/config"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

type customizedMetric struct {
}

const Win_Rerf_Counters_Key = "win_perf_counters"

func GetObjectPath(object string) string {
	curPath := parent.GetCurPath() + Win_Rerf_Counters_Key + "/" + object + "/"
	return curPath
}

// This rule is specifically for windows customized metrics
func (c *customizedMetric) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	win_Perf_Counters_Array := []interface{}{}

	inputmap := input.(map[string]interface{})
	inputObjectNames := []string{}
	for objectName := range inputmap {
		if config.DisableWinPerfCounters[objectName] {
			continue
		}
		inputObjectNames = append(inputObjectNames, objectName)
	}
	sort.Strings(inputObjectNames)
	for _, objectName := range inputObjectNames {
		singleConfig := util.ProcessWindowsCommonConfig(inputmap[objectName], objectName, GetObjectPath(objectName))
		win_Perf_Counters_Array = addToWinPerfArray(singleConfig, win_Perf_Counters_Array)
	}

	if len(win_Perf_Counters_Array) != 0 {
		//Keep the windows metrics outcome consistent
		for _, perfC := range win_Perf_Counters_Array {
			objectConfig := util.MetricArray{}
			mp := perfC.(map[string]interface{})
			objectConfig = append(objectConfig, mp["object"].([]interface{})...)
			sort.Sort(objectConfig)
			mp["object"] = objectConfig
		}
		returnKey = Win_Rerf_Counters_Key
		returnVal = win_Perf_Counters_Array
	}
	return
}

func addToWinPerfArray(input interface{}, win_Perf_Counters_Array []interface{}) []interface{} {
	inputmap := input.(map[string]interface{})
	var input_interval string
	var input_tag interface{}
	var input_object interface{}
	// extract "key" to see if this win_perf_counters can be merged with others

	// extract interval:
	if val, ok := inputmap["interval"]; ok {
		input_interval = val.(string)
	}

	// extract tags:
	if val, ok := inputmap["tags"]; ok {
		input_tag = val
	}

	// extract input object
	if val, ok := inputmap["object"]; ok {
		input_object = val
	}

	// check if this can be merged with existing win_perf_counters
	// otherwise create an new entry
	if !mergeIfMatch(input_interval, input_tag, input_object, win_Perf_Counters_Array) {
		return append(win_Perf_Counters_Array, input)
	}
	return win_Perf_Counters_Array
}

func mergeIfMatch(inputInterval string, inputTags interface{}, inputObject interface{}, win_Perf_Counters_Array []interface{}) bool {
	for _, perf_counter := range win_Perf_Counters_Array {
		var target_interval string
		var target_tags interface{}
		pm := perf_counter.(map[string]interface{})
		if val, ok := pm["interval"]; ok {
			target_interval = val.(string)
		}
		if val, ok := pm["tags"]; ok {
			target_tags = val
		}
		if target_interval == inputInterval && reflect.DeepEqual(inputTags, target_tags) {
			pm["object"] = append(pm["object"].([]interface{}), inputObject.([]interface{})...)
			return true
		}
	}
	return false
}

func init() {
	parent.RegisterWindowsRule("customizedMetric", new(customizedMetric))
}
