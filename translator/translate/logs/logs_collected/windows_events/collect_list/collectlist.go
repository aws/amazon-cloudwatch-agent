// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collectlist

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonRule"
	"github.com/aws/amazon-cloudwatch-agent/translator/jsonconfig/mergeJsonUtil"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/logs_collected/windows_events"
	logUtil "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/util"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

type Rule translator.Rule

const (
	SectionKey         = "collect_list"
	EventConfigTomlKey = "event_config"
	BatchReadSizeKey   = "batch_read_size"
	EventLevelsKey     = "event_levels"
	EventIDKey         = "event_ids"
	//TODO: Performance test to confirm the proper value here - https://github.com/aws/amazon-cloudwatch-agent/issues/231
	BatchReadSizeValue = 170
)

var ChildRule = map[string]Rule{}

func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

type CollectList struct {
}

var customizedJSONConfigKeys = []string{"event_name", EventLevelsKey, EventIDKey}
var eventLevelMapping = map[string]string{
	"VERBOSE":     "5",
	"INFORMATION": "4",
	"WARNING":     "3",
	"ERROR":       "2",
	"CRITICAL":    "1",
}

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func (c *CollectList) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := []interface{}{}

	if _, ok := im[SectionKey]; ok {
		for _, singleConfig := range im[SectionKey].([]interface{}) {
			singleTransformedConfig := getTransformedConfig(singleConfig)
			result = append(result, singleTransformedConfig)
		}
	}
	logUtil.ValidateLogGroupFields(result, GetCurPath())
	return EventConfigTomlKey, result
}

var MergeRuleMap = map[string]mergeJsonRule.MergeRule{}

func (c *CollectList) Merge(source map[string]interface{}, result map[string]interface{}) {
	mergeJsonUtil.MergeList(source, result, SectionKey)
}

func init() {
	obj := new(CollectList)
	parent.RegisterRule("windowseventlog_collectList", obj)
	parent.MergeRuleMap[SectionKey] = obj
}

func getTransformedConfig(input interface{}) interface{} {
	result := map[string]interface{}{}
	// Extract customer specified config
	util.SetWithSameKeyIfFound(input, customizedJSONConfigKeys, result)
	// Set Fixed config
	addFixedJsonConfig(result)

	for _, rule := range ChildRule {
		key, val := rule.ApplyRule(input)
		if key != "" {
			result[key] = val
		}
	}

	return result
}

func addFixedJsonConfig(result map[string]interface{}) {
	result[BatchReadSizeKey] = BatchReadSizeValue

	var inputEventLevels []interface{}
	if eventLevels, ok := result[EventLevelsKey]; !ok {
		return
	} else {
		inputEventLevels = eventLevels.([]interface{})
	}
	resultEventLevels := []interface{}{}
	for _, eventLevel := range inputEventLevels {
		switch eventLevel.(string) {
		case "CRITICAL":
			resultEventLevels = append(resultEventLevels, "1")
		case "ERROR":
			resultEventLevels = append(resultEventLevels, "2")
		case "WARNING":
			resultEventLevels = append(resultEventLevels, "3")
		case "INFORMATION":
			resultEventLevels = append(resultEventLevels, "4", "0")
		case "VERBOSE":
			resultEventLevels = append(resultEventLevels, "5")
		default:
			translator.AddErrorMessages(GetCurPath(), fmt.Sprintf("Cannot find the mapping for Windows event level %v.", eventLevel))
		}
	}
	result[EventLevelsKey] = resultEventLevels

	if eventIDs, ok := result[EventIDKey]; ok {
		validatedIDs, errorMessages := validateEventIDs(eventIDs.([]interface{}))
		for _, err := range errorMessages {
			translator.AddErrorMessages(GetCurPath(), err)
		}
		result[EventIDKey] = validatedIDs
	}
}

// Validate event_id inputs
func validateEventIDs(inputEventIDs []interface{}) ([]int, []string) {
	validatedIDs := []int{}
	errorMessages := []string{}

	const (
		minEventID = 0
		maxEventID = 65535
	)

	for _, id := range inputEventIDs {
		eventIDFloat, ok := id.(float64)
		if !ok {
			errorMessages = append(errorMessages, fmt.Sprintf("Event ID %v is not a number", id))
			continue
		}
		eventIDInt := int(eventIDFloat)
		if eventIDInt < minEventID || eventIDInt > maxEventID {
			errorMessages = append(errorMessages, fmt.Sprintf("Event ID %v is not a valid windows event_id.", eventIDInt))
			continue
		}

		validatedIDs = append(validatedIDs, eventIDInt)
	}

	return validatedIDs, errorMessages

}
