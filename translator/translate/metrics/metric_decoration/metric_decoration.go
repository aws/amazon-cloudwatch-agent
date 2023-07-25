// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metric_decoration

import (
	"sort"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/util"
)

const SectionKey = "metric_decoration"

type MetricDecoration struct {
}

func (m *MetricDecoration) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := []interface{}{}

	targetOs := translator.GetTargetPlatform()
	if targetOs == "" {
		targetOs = config.OS_TYPE_LINUX
	}

	//Check if metrics_collect.SectionKey exist in the input instance
	//If not, not process
	if section, ok := im[metrics_collect.SectionKey]; ok {
		pluginMap := section.(map[string]interface{})
		//If yes, process it
		// sort the key first
		sortedKey := []string{}
		for k := range pluginMap {
			sortedKey = append(sortedKey, k)
		}

		sort.Strings(sortedKey)
		for _, key := range sortedKey {
			/** handle different types: array and map.
			 * array means multiple plugins
			 * array example:
			 * {"procstat": [{...}, {...}]}
			 *
			 * map example:
			 * {"cpu": {...}}
			 **/
			switch pluginMap[key].(type) {
			case map[string]interface{}:
				plugin := pluginMap[key].(map[string]interface{})
				if _, ok = plugin[util.Measurement_Key]; !ok {
					continue
				}

				decorations := util.ApplyMeasurementRuleForMetricDecoration(plugin[util.Measurement_Key], key, targetOs)
				result = append(result, decorations...)
			case []map[string]interface{}:
				plugins := pluginMap[key].([]map[string]interface{})
				for _, plugin := range plugins {
					if _, ok = plugin[util.Measurement_Key]; !ok {
						continue
					}
					decorations := util.ApplyMeasurementRuleForMetricDecoration(plugin[util.Measurement_Key], key, targetOs)
					result = append(result, decorations...)
				}
			}

		}
	}

	returnKey = SectionKey
	returnVal = result
	return
}

func init() {
	m := new(MetricDecoration)
	parent.RegisterRule(SectionKey, m)
}
