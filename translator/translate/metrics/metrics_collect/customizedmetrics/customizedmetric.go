// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package customizedmetrics

import (
	"sort"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/config"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

type customizedMetric struct {
}

const WinPerfCountersKey = "win_perf_counters"

func GetObjectPath(object string) string {
	curPath := parent.GetCurPath() + WinPerfCountersKey + "/" + object + "/"
	return curPath
}

// This rule is specifically for windows customized metrics
func (c *customizedMetric) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	winPerfCountersArray := []interface{}{}

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
		winPerfCountersArray = append(winPerfCountersArray, singleConfig)
	}

	if len(winPerfCountersArray) != 0 {
		//Keep the windows metrics outcome consistent
		for _, perfC := range winPerfCountersArray {
			objectConfig := util.MetricArray{}
			mp := perfC.(map[string]interface{})
			objectConfig = append(objectConfig, mp["object"].([]interface{})...)
			sort.Sort(objectConfig)
			mp["object"] = objectConfig
		}
		returnKey = WinPerfCountersKey
		returnVal = winPerfCountersArray
	}
	return
}

func init() {
	parent.RegisterWindowsRule("customizedMetric", new(customizedMetric))
}
