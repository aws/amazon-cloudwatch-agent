// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package drop_origin

import (
	"log"
	"reflect"

	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics/metrics_collect"
)

type DropOrigin struct {
}

const SectionKey = "drop_original_metrics"

func (do *DropOrigin) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})

	returnKey = ""
	returnVal = ""
	if _, ok := im[metrics_collect.SectionKey]; !ok {
		return
	} else {
		pluginMap := im[metrics_collect.SectionKey].(map[string]interface{})
		result := make(map[string][]string)

		for key, val := range pluginMap {
			if entries, isMap := val.(map[string]interface{}); isMap {
				if values, isSlice := entries[SectionKey].([]interface{}); isSlice && len(values) > 0 {
					droppingDimensions := make([]string, 0)
					for _, value := range values {
						if reflect.TypeOf(value).String() == "string" {
							droppingDimensions = append(droppingDimensions, value.(string))
						} else {
							log.Panic("Fail to translate the JSON config, invalid format of dropping dimensions.")
						}
					}

					returnKey = SectionKey
					result[config.GetRealPluginName(key)] = droppingDimensions
					returnVal = result
				}
			}
		}
	}
	return
}

func init() {
	do := new(DropOrigin)
	parent.RegisterRule(SectionKey, do)
}
